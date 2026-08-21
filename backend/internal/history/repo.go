package history

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Upsert(row *WatchHistory) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"position_ms", "duration_ms", "completed", "watched_at_ms"}),
	}).Create(row).Error
}

func (r *Repo) Find(accountID, videoID uint64) (*WatchHistory, error) {
	var row WatchHistory
	err := r.db.Where("account_id = ? AND video_id = ?", accountID, videoID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) FindByVideoIDs(accountID uint64, videoIDs []uint64) ([]WatchHistory, error) {
	if len(videoIDs) == 0 {
		return []WatchHistory{}, nil
	}
	var rows []WatchHistory
	if err := r.db.Where("account_id = ? AND video_id IN ?", accountID, videoIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		return []WatchHistory{}, nil
	}
	return rows, nil
}

func (r *Repo) List(accountID uint64, completed bool, cursorMs int64, cursorID uint64, limit int) ([]WatchHistory, error) {
	query := r.db.Where("account_id = ? AND completed = ?", accountID, completed)
	if cursorMs > 0 || cursorID > 0 {
		query = query.Where(
			"watched_at_ms < ? OR (watched_at_ms = ? AND id < ?)",
			cursorMs, cursorMs, cursorID,
		)
	}
	var rows []WatchHistory
	if err := query.Order("watched_at_ms DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		return []WatchHistory{}, nil
	}
	return rows, nil
}
