package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"my_feed_system/internal/mq"
	"my_feed_system/internal/outbox"
)

const defaultMaxVideoBytes int64 = 256 << 20

var (
	ErrVideoUploadEmpty    = errors.New("video file is empty")
	ErrVideoUploadTooLarge = errors.New("video file is too large")
)

type Service struct {
	db            *gorm.DB
	repo          *Repo
	outboxRepo    *outbox.Repo
	uploadDir     string
	maxVideoBytes int64
	partsMu       sync.Mutex
	parts         map[string]*partSession
}

func NewService(db *gorm.DB, uploadDir string, maxVideoBytes int64) *Service {
	if maxVideoBytes <= 0 {
		maxVideoBytes = defaultMaxVideoBytes
	}
	return &Service{
		db:            db,
		repo:          NewRepo(db),
		outboxRepo:    outbox.NewRepo(db),
		uploadDir:     uploadDir,
		maxVideoBytes: maxVideoBytes,
		parts:         make(map[string]*partSession),
	}
}

func (s *Service) CreateVideoTask(accountID uint64, file *multipart.FileHeader) (*Task, error) {
	if file == nil || file.Size <= 0 {
		return nil, ErrVideoUploadEmpty
	}
	if file.Size > s.maxVideoBytes {
		return nil, ErrVideoUploadTooLarge
	}

	sourceDir := filepath.Join(s.uploadDir, "sources")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create media source directory: %w", err)
	}

	sourcePath := filepath.Join(sourceDir, fmt.Sprintf("%d_%s.source", accountID, randomSuffix()))
	if err := saveLimitedFile(file, sourcePath, s.maxVideoBytes); err != nil {
		_ = os.Remove(sourcePath)
		return nil, err
	}

	return s.enqueueVideoTask(accountID, sourcePath)
}

func (s *Service) enqueueVideoTask(accountID uint64, sourcePath string) (*Task, error) {
	task := &Task{
		AccountID:   accountID,
		SourcePath:  sourcePath,
		ContentType: "video/mp4",
		Status:      StatusProcessing,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Create(tx, task); err != nil {
			return err
		}
		event, err := mq.NewEnvelope(mq.EventTypeMediaTranscodeRequested, mq.ProducerAPIServer, mq.MediaTranscodePayload{
			TaskID:    task.ID,
			AccountID: accountID,
		})
		if err != nil {
			return fmt.Errorf("build media.transcode.requested event: %w", err)
		}
		if err := s.outboxRepo.Enqueue(tx, event); err != nil {
			return fmt.Errorf("enqueue media.transcode.requested event: %w", err)
		}
		return nil
	}); err != nil {
		_ = os.Remove(sourcePath)
		return nil, err
	}

	return task, nil
}

func (s *Service) GetTask(accountID uint64, taskID uint64) (*Task, error) {
	return s.repo.FindByIDForAccount(taskID, accountID)
}

func (s *Service) MaxVideoBytes() int64 {
	return s.maxVideoBytes
}

func saveLimitedFile(file *multipart.FileHeader, targetPath string, maxBytes int64) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open uploaded video: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("create uploaded video: %w", err)
	}

	written, copyErr := io.Copy(dst, io.LimitReader(src, maxBytes+1))
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("save uploaded video: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close uploaded video: %w", closeErr)
	}
	if written == 0 {
		return ErrVideoUploadEmpty
	}
	if written > maxBytes {
		return ErrVideoUploadTooLarge
	}
	return nil
}

func randomSuffix() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
}
