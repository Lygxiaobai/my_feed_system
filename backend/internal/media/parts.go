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

// UploadPartBytes 是经 Cloudflare 时的默认单段大小。回源大约 100 秒，段必须能在窗口内走完。
// 256KiB×2 等于一次只推约 512KB，体感很慢；1MiB×2 仍低于此前失败的 2MiB×4。
const UploadPartBytes int64 = 1 << 20

// UploadPartMaxBytes 是直连源站时的单段上限。灰云/对象存储不再受 100 秒窗口限制。
const UploadPartMaxBytes int64 = 4 << 20

// UploadPartConcurrency 经 Cloudflare 时的并行路上限。再高会把单段拖过 100 秒。
const UploadPartConcurrency = 2

// UploadPartDirectConcurrency 直连源站时的并行路数。
const UploadPartDirectConcurrency = 4

const (
	maxPartSessionsPerAccount = 2
	partSessionTTL            = 2 * time.Hour
	partMaxSlack              = 64 << 10
)

var (
	ErrUploadSessionNotFound = errors.New("upload session not found")
	ErrUploadSessionConflict = errors.New("upload session conflict")
	ErrTooManyUploadSessions = errors.New("too many upload sessions")
)

type partSession struct {
	mu         sync.Mutex
	ID         string
	AccountID  uint64
	Total      int64
	PartBytes  int64
	Count      int
	Dir        string
	got        map[int]int64
	assembling bool
	CreatedAt  time.Time
}

func partCountForSize(totalSize int64, partBytes int64) int {
	if totalSize <= 0 || partBytes <= 0 {
		return 0
	}
	return int((totalSize + partBytes - 1) / partBytes)
}

// BeginUpload 先用一个小 JSON 建会话。各段之后按序号各写各的文件，允许并行到达。
func (s *Service) BeginUpload(accountID uint64, totalSize int64, partBytes int64) (string, int, error) {
	if totalSize <= 0 || totalSize > s.maxVideoBytes {
		return "", 0, ErrVideoUploadTooLarge
	}
	if partBytes <= 0 || partBytes > UploadPartMaxBytes {
		partBytes = UploadPartBytes
	}
	count := partCountForSize(totalSize, partBytes)
	if count <= 0 {
		return "", 0, ErrVideoUploadEmpty
	}

	s.partsMu.Lock()
	defer s.partsMu.Unlock()
	s.expirePartsLocked(time.Now())
	// 重试和换文件都会再走 BeginUpload，没有单独的取消接口。
	// 失败留下的空会话不能占满名额一直等到 TTL。
	s.replaceAbandonedAccountPartsLocked(accountID)
	if s.countAccountPartsLocked(accountID) >= maxPartSessionsPerAccount {
		return "", 0, ErrTooManyUploadSessions
	}

	id := randomSuffix() + randomSuffix()
	dir := filepath.Join(s.uploadDir, "parts", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create upload session directory: %w", err)
	}

	s.parts[id] = &partSession{
		ID:        id,
		AccountID: accountID,
		Total:     totalSize,
		PartBytes: partBytes,
		Count:     count,
		Dir:       dir,
		got:       make(map[int]int64, count),
		CreatedAt: time.Now(),
	}
	return id, count, nil
}

// PutUploadPart 写入指定序号的一段。同一段重复到达则覆盖，收齐后拼成源文件并建转码任务。
func (s *Service) PutUploadPart(accountID uint64, sessionID string, index int, count int, data io.Reader, dataLen int64) (int64, *Task, error) {
	if sessionID == "" {
		return 0, nil, ErrUploadSessionNotFound
	}
	if dataLen <= 0 || dataLen > UploadPartMaxBytes+partMaxSlack {
		return 0, nil, ErrUploadSessionConflict
	}

	s.partsMu.Lock()
	s.expirePartsLocked(time.Now())
	sess := s.parts[sessionID]
	if sess == nil || sess.AccountID != accountID {
		s.partsMu.Unlock()
		return 0, nil, ErrUploadSessionNotFound
	}
	if count != sess.Count || index < 0 || index >= sess.Count {
		s.partsMu.Unlock()
		return 0, nil, ErrUploadSessionConflict
	}
	dir := sess.Dir
	s.partsMu.Unlock()

	partPath := filepath.Join(dir, fmt.Sprintf("%08d.part", index))
	tmpPath := partPath + ".tmp"
	if err := writeLimitedFile(tmpPath, data, dataLen); err != nil {
		_ = os.Remove(tmpPath)
		return 0, nil, err
	}
	if err := os.Rename(tmpPath, partPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, nil, fmt.Errorf("commit upload part: %w", err)
	}

	s.partsMu.Lock()
	sess = s.parts[sessionID]
	if sess == nil || sess.AccountID != accountID {
		s.partsMu.Unlock()
		return 0, nil, ErrUploadSessionNotFound
	}
	sess.mu.Lock()
	sess.got[index] = dataLen
	received := receivedLocked(sess)
	complete := len(sess.got) == sess.Count && !sess.assembling
	if complete {
		sess.assembling = true
	}
	sess.mu.Unlock()
	s.partsMu.Unlock()

	if !complete {
		return received, nil, nil
	}

	task, err := s.assembleSession(sess)
	s.partsMu.Lock()
	delete(s.parts, sessionID)
	s.partsMu.Unlock()
	if err != nil {
		_ = os.RemoveAll(sess.Dir)
		return received, nil, err
	}
	return received, task, nil
}

func receivedLocked(sess *partSession) int64 {
	var n int64
	for _, size := range sess.got {
		n += size
	}
	return n
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

func (s *Service) assembleSession(sess *partSession) (*Task, error) {
	merged := filepath.Join(sess.Dir, "merged.source")
	out, err := os.OpenFile(merged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, fmt.Errorf("create merged upload: %w", err)
	}

	var written int64
	for i := 0; i < sess.Count; i++ {
		in, openErr := os.Open(filepath.Join(sess.Dir, fmt.Sprintf("%08d.part", i)))
		if openErr != nil {
			_ = out.Close()
			return nil, fmt.Errorf("open upload part %d: %w", i, openErr)
		}
		n, copyErr := io.Copy(out, in)
		_ = in.Close()
		if copyErr != nil {
			_ = out.Close()
			return nil, fmt.Errorf("merge upload part %d: %w", i, copyErr)
		}
		written += n
	}
	if err := out.Close(); err != nil {
		return nil, fmt.Errorf("close merged upload: %w", err)
	}
	if written != sess.Total {
		return nil, fmt.Errorf("merged size %d != %d: %w", written, sess.Total, ErrUploadSessionConflict)
	}

	task, err := s.createVideoTaskFromSource(sess.AccountID, merged)
	_ = os.RemoveAll(sess.Dir)
	return task, err
}

func (s *Service) replaceAbandonedAccountPartsLocked(accountID uint64) {
	for id, sess := range s.parts {
		if sess.AccountID != accountID {
			continue
		}
		sess.mu.Lock()
		busy := sess.assembling
		sess.mu.Unlock()
		if busy {
			continue
		}
		delete(s.parts, id)
		_ = os.RemoveAll(sess.Dir)
	}
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
		_ = os.RemoveAll(sess.Dir)
	}
}

func writeLimitedFile(path string, data io.Reader, n int64) error {
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
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
