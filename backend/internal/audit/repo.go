package audit

import (
	"time"

	"gorm.io/gorm"
)

// Repo 负责审核流水的读写。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// Append 追加一条审核流水。必须与状态变更在同一事务里，
// 否则会出现「状态已变但没有记录」或反之的情况，事后无法追溯。
func (r *Repo) Append(tx *gorm.DB, record *Record) error {
	return tx.Create(record).Error
}

// ListByTarget 返回某个内容的完整处置链路，供管理端排查。
func (r *Repo) ListByTarget(targetType TargetType, targetID uint64) ([]Record, error) {
	var rows []Record
	err := r.db.
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}

// PurgeBefore 清理早于指定时间的流水。
//
// 刻意不提供「保留 N 天」的便捷封装：留存期是合规要求而非技术偏好，
// 调用方必须显式传入时间点，避免有人随手改小。
func (r *Repo) PurgeBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", cutoff).Delete(&Record{})
	return result.RowsAffected, result.Error
}
