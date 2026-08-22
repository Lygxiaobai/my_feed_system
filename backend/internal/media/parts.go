package media

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UploadPartBytes 是单段上限。整文件一次过 Cloudflare 会撞上约 100 秒源站超时，
// 4MiB 在常见上行速度下几秒就能结束，远低于那条硬限制。
const UploadPartBytes int64 = 4 << 20

const (
	maxPartSessionsPerAccount = 2
	partSessionTTL            = 30 * time.Minute
	partMaxSlack              = 64 << 10
)

var (
	ErrUploadSessionNotFound = errors.New("upload session not found")
	ErrUploadSessionConflict = errors.New("upload session conflict")
	ErrTooManyUploadSessions = errors.New("too many upload sessions")
)

type partSession struct {
	mu        sync.Mutex
	ID        string
	AccountID uint64
	Total     int64
	Received  int64
	Path      string
	CreatedAt time.Time
}

// AppendUploadPart 按顺序把一段字节追加到账号自己的临时文件。
// 收齐后才创建转码任务，中间段不占「投稿次数」配额。
func (s *Service) AppendUploadPart(accountID uint64, sessionID string, totalSize int64, data io.Reader, dataLen int64) (string, int64, *Task, error) {
	if totalSize <= 0 || totalSize > s.maxVideoBytes {
		return "", 0, nil, ErrVideoUploadTooLarge
	}
	if dataLen <= 0 || dataLen > UploadPartBytes+partMaxSlack {
		return "", 0, nil, ErrUploadSessionConflict
	}

	s.partsMu.Lock()
	s.expirePartsLocked(time.Now())

	var sess *partSession
	if sessionID == "" {
		if s.countAccountPartsLocked(accountID) >= maxPartSessionsPerAccount {
			s.partsMu.Unlock()
			return "", 0, nil, ErrTooManyUploadSessions
		}
		id := randomSuffix() + randomSuffix()
		partDir := filepath.Join(s.uploadDir, "parts")
		if err := os.MkdirAll(partDir, 0o755); err != nil {
			s.partsMu.Unlock()
			return "", 0, nil, fmt.Errorf("create upload part directory: %w", err)
		}
		sess = &partSession{
			ID:        id,
			AccountID: accountID,
			Total:     totalSize,
			Path:      filepath.Join(partDir, id+".part"),
			CreatedAt: time.Now(),
		}
		s.parts[id] = sess
	} else {
		sess = s.parts[sessionID]
		if sess == nil || sess.AccountID != accountID {
			s.partsMu.Unlock()
			return "", 0, nil, ErrUploadSessionNotFound
		}
		if sess.Total != totalSize {
			s.partsMu.Unlock()
			return "", 0, nil, ErrUploadSessionConflict
		}
	}

	sess.mu.Lock()
	s.partsMu.Unlock()

	if sess.Received+dataLen > sess.Total {
		sess.mu.Unlock()
		return "", 0, nil, ErrUploadSessionConflict
	}

	if err := appendLimitedFile(sess.Path, data, dataLen); err != nil {
		sess.mu.Unlock()
		return sess.ID, sess.Received, nil, err
	}
	sess.Received += dataLen
	received := sess.Received
	complete := received >= sess.Total
	id := sess.ID
	sourcePath := sess.Path
	sess.mu.Unlock()

	if !complete {
		return id, received, nil, nil
	}

	s.partsMu.Lock()
	delete(s.parts, id)
	s.partsMu.Unlock()

	task, err := s.createVideoTaskFromSource(accountID, sourcePath)
	if err != nil {
		_ = os.Remove(sourcePath)
		return id, received, nil, err
	}
	return id, received, task, nil
}

func (s *Service) createVideoTaskFromSource(accountID uint64, sourcePath string) (*Task, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat assembled video: %w", err)
	}
	if info.Size() <= 0 {
		return nil, ErrVideoUploadEmpty
	}
	if info.Size() > s.maxVideoBytes {
		return nil, ErrVideoUploadTooLarge
	}

	finalDir := filepath.Join(s.uploadDir, "sources")
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return nil, fmt.Errorf("create media source directory: %w", err)
	}
	finalPath := filepath.Join(finalDir, fmt.Sprintf("%d_%s.source", accountID, randomSuffix()))
	if err := os.Rename(sourcePath, finalPath); err != nil {
		return nil, fmt.Errorf("promote uploaded video: %w", err)
	}
	return s.enqueueVideoTask(accountID, finalPath)
}

func (s *Service) countAccountPartsLocked(accountID uint64) int {
	n := 0
	for _, sess := range s.parts {
		if sess.AccountID == accountID {
			n++
		}
	}
	return n
}

func (s *Service) expirePartsLocked(now time.Time) {
	for id, sess := range s.parts {
		if now.Sub(sess.CreatedAt) < partSessionTTL {
			continue
		}
		delete(s.parts, id)
		_ = os.Remove(sess.Path)
	}
}

func appendLimitedFile(path string, data io.Reader, n int64) error {
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("open upload part: %w", err)
	}
	written, copyErr := io.Copy(dst, io.LimitReader(data, n))
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("write upload part: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close upload part: %w", closeErr)
	}
	if written != n {
		return fmt.Errorf("write upload part: short write")
	}
	return nil
}
