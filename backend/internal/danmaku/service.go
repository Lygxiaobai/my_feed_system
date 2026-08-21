package danmaku

import (
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"my_feed_system/internal/video"
)

var (
	ErrEmptyContent   = errors.New("danmaku content is empty")
	ErrContentTooLong = errors.New("danmaku content is too long")
	ErrInvalidOffset  = errors.New("danmaku offset is invalid")
)

type Service struct {
	repo   *Repo
	videos VideoAccess
}

func NewService(db *gorm.DB, videos VideoAccess) *Service {
	return &Service{
		repo:   NewRepo(db),
		videos: videos,
	}
}

func (s *Service) List(viewerID uint64, req ListRequest) ([]VideoDanmaku, error) {
	if err := s.videos.EnsureVisible(viewerID, req.VideoID); err != nil {
		return nil, err
	}
	return s.repo.ListByVideoID(req.VideoID, MaxListSize)
}

func (s *Service) Send(accountID uint64, username string, req SendRequest) (*VideoDanmaku, error) {
	content, err := normalizeContent(req.Content)
	if err != nil {
		return nil, err
	}
	if req.OffsetMs < 0 || req.OffsetMs > MaxOffsetMs {
		return nil, ErrInvalidOffset
	}
	if err := s.videos.EnsureVisible(accountID, req.VideoID); err != nil {
		return nil, err
	}

	row := &VideoDanmaku{
		VideoID:  req.VideoID,
		AuthorID: accountID,
		Username: strings.TrimSpace(username),
		Content:  content,
		OffsetMs: req.OffsetMs,
	}
	if err := s.repo.Create(row); err != nil {
		return nil, err
	}
	return row, nil
}

func normalizeContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	if content == "" {
		return "", ErrEmptyContent
	}
	if utf8.RuneCountInString(content) > MaxContentRunes {
		return "", ErrContentTooLong
	}
	return content, nil
}

func isVideoMissing(err error) bool {
	return errors.Is(err, video.ErrVideoNotFound)
}
