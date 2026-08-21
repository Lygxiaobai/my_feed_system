package history

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"my_feed_system/internal/video"
)

var (
	ErrInvalidProgress = errors.New("watch progress is invalid")
	ErrInvalidStatus   = errors.New("history status is invalid")
	ErrInvalidCursor   = errors.New("history cursor is invalid")
	ErrTooManyIDs      = errors.New("too many video ids")
)

type videoLookup interface {
	GetDetail(viewerID uint64, req video.GetDetailRequest) (*video.Video, error)
}

type Service struct {
	repo   *Repo
	videos videoLookup
}

func NewService(db *gorm.DB, videos videoLookup) *Service {
	return &Service{
		repo:   NewRepo(db),
		videos: videos,
	}
}

type UpsertResult struct {
	Saved bool         `json:"saved"`
	Item  *HistoryItem `json:"item"`
}

func (s *Service) Upsert(accountID uint64, req UpsertRequest) (*UpsertResult, error) {
	if req.VideoID == 0 {
		return nil, ErrInvalidProgress
	}
	if req.PositionMs < 0 || req.DurationMs < 0 || req.PositionMs > MaxOffsetMs || req.DurationMs > MaxOffsetMs {
		return nil, ErrInvalidProgress
	}
	if _, err := s.videos.GetDetail(accountID, video.GetDetailRequest{ID: req.VideoID}); err != nil {
		return nil, err
	}

	position := clampPosition(req.PositionMs, req.DurationMs)
	if !ShouldPersist(position, req.DurationMs) {
		return &UpsertResult{Saved: false}, nil
	}

	completed := IsCompleted(position, req.DurationMs)
	storedPosition := position
	if completed {
		// 已看完把进度清零，下次打开从头播，也避免循环回零被写成「没看过」。
		storedPosition = 0
	}

	row := &WatchHistory{
		AccountID:   accountID,
		VideoID:     req.VideoID,
		PositionMs:  storedPosition,
		DurationMs:  req.DurationMs,
		Completed:   completed,
		WatchedAtMs: time.Now().UnixMilli(),
	}
	if err := s.repo.Upsert(row); err != nil {
		return nil, err
	}
	item, err := s.itemFromRow(accountID, row)
	if err != nil {
		return nil, err
	}
	return &UpsertResult{Saved: true, Item: item}, nil
}

type ListResult struct {
	Items      []HistoryItem `json:"items"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

func (s *Service) List(accountID uint64, req ListRequest) (*ListResult, error) {
	completed, err := parseStatus(req.Status)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	cursorMs, cursorID, err := parseCursor(req.Cursor)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.List(accountID, completed, cursorMs, cursorID, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]HistoryItem, 0, len(rows))
	for i := range rows {
		item, err := s.itemFromRow(accountID, &rows[i])
		if err != nil {
			if errors.Is(err, video.ErrVideoNotFound) {
				continue
			}
			return nil, err
		}
		if item == nil {
			continue
		}
		items = append(items, *item)
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeCursor(last.WatchedAtMs, last.ID)
	}
	return &ListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) Progress(accountID uint64, req ProgressRequest) ([]ProgressItem, error) {
	if len(req.VideoIDs) == 0 {
		return []ProgressItem{}, nil
	}
	if len(req.VideoIDs) > MaxProgressIDs {
		return nil, ErrTooManyIDs
	}

	visible := make([]uint64, 0, len(req.VideoIDs))
	seen := make(map[uint64]struct{}, len(req.VideoIDs))
	for _, id := range req.VideoIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, err := s.videos.GetDetail(accountID, video.GetDetailRequest{ID: id}); err != nil {
			if errors.Is(err, video.ErrVideoNotFound) {
				continue
			}
			return nil, err
		}
		visible = append(visible, id)
	}

	rows, err := s.repo.FindByVideoIDs(accountID, visible)
	if err != nil {
		return nil, err
	}
	items := make([]ProgressItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ProgressItem{
			VideoID:    row.VideoID,
			PositionMs: row.PositionMs,
			DurationMs: row.DurationMs,
			Completed:  row.Completed,
			ResumeMs:   ResumeMs(row.PositionMs, row.DurationMs, row.Completed),
		})
	}
	return items, nil
}

func (s *Service) itemFromRow(accountID uint64, row *WatchHistory) (*HistoryItem, error) {
	v, err := s.videos.GetDetail(accountID, video.GetDetailRequest{ID: row.VideoID})
	if err != nil {
		return nil, err
	}
	return &HistoryItem{
		VideoID:       row.VideoID,
		PositionMs:    row.PositionMs,
		DurationMs:    row.DurationMs,
		Completed:     row.Completed,
		ResumeMs:      ResumeMs(row.PositionMs, row.DurationMs, row.Completed),
		LastWatchedAt: time.UnixMilli(row.WatchedAtMs).Format(time.RFC3339),
		Video:         v,
	}, nil
}

func parseStatus(status string) (bool, error) {
	switch strings.TrimSpace(status) {
	case StatusUnfinished:
		return false, nil
	case StatusCompleted:
		return true, nil
	default:
		return false, ErrInvalidStatus
	}
}

func encodeCursor(ms int64, id uint64) string {
	return fmt.Sprintf("%d:%d", ms, id)
}

func parseCursor(raw string) (int64, uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, nil
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, 0, ErrInvalidCursor
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || ms < 0 {
		return 0, 0, ErrInvalidCursor
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	return ms, id, nil
}
