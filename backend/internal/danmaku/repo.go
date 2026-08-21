package danmaku

import "gorm.io/gorm"

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(row *VideoDanmaku) error {
	return r.db.Create(row).Error
}

func (r *Repo) ListByVideoID(videoID uint64, limit int) ([]VideoDanmaku, error) {
	if limit <= 0 {
		limit = MaxListSize
	}
	var rows []VideoDanmaku
	err := r.db.Where("video_id = ?", videoID).
		Order("offset_ms ASC, id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return []VideoDanmaku{}, nil
	}
	return rows, nil
}
