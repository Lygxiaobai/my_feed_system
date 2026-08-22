package report

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// Create 插入一条举报。
//
// 返回 inserted=false 表示该举报人已举报过同一对象。
// 用 OnConflict DoNothing 让唯一索引来判定重复，而不是「先 SELECT 再 INSERT」——
// 后者在并发下必然漏判。
func (r *Repo) Create(row *Report) (inserted bool, err error) {
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ListByReporter 返回某个举报人提交过的举报，按时间倒序。
func (r *Repo) ListByReporter(reporterID uint64, limit int, offsetID uint64) ([]Report, error) {
	query := r.db.Where("reporter_id = ?", reporterID)
	if offsetID > 0 {
		query = query.Where("id < ?", offsetID)
	}

	var rows []Report
	if err := query.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListPending 返回所有待处理的举报，按提交时间正序。
//
// 不在 SQL 里做 GROUP BY 聚合：审核员还需要看到理由分布与补充说明原文，
// 聚合后这些信息就没了，得再查一次明细。待处理量级很小（人工处理的上限就摆在那），
// 直接取明细在内存里归并更简单，也少一次往返。
func (r *Repo) ListPending(limit int) ([]Report, error) {
	var rows []Report
	if err := r.db.
		Where("status = ?", StatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CountPending 返回当前全部待处理举报条数（不是对象数）。
func (r *Repo) CountPending() (int64, error) {
	var count int64
	if err := r.db.Model(&Report{}).Where("status = ?", StatusPending).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountPendingByTarget 返回某个对象当前待处理的举报条数。
func (r *Repo) CountPendingByTarget(targetType TargetType, targetID uint64) (int64, error) {
	var count int64
	if err := r.db.Model(&Report{}).
		Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, StatusPending).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ResolveTarget 把某个对象下所有待处理的举报一次性结案。
//
// 只更新 pending 的行：已经结过案的历史举报不该被后续处置覆盖，
// 否则追溯时会看到被改写过的结论。
func (r *Repo) ResolveTarget(tx *gorm.DB, targetType TargetType, targetID uint64, status Status, handlerID uint64, note string, handledAt time.Time) (int64, error) {
	db := tx
	if db == nil {
		db = r.db
	}

	result := db.Model(&Report{}).
		Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, StatusPending).
		Updates(map[string]any{
			"status":      status,
			"handler_id":  handlerID,
			"handle_note": note,
			"handled_at":  handledAt,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// FindByTargetAndReporter 读取某人对某对象的举报，不存在时返回 nil。
func (r *Repo) FindByTargetAndReporter(targetType TargetType, targetID uint64, reporterID uint64) (*Report, error) {
	var row Report
	err := r.db.
		Where("target_type = ? AND target_id = ? AND reporter_id = ?", targetType, targetID, reporterID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
