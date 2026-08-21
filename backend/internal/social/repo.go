package social

import (
	"errors"

	"gorm.io/gorm"
)

// Repo 负责关注关系表的查询和写入。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) FindByPair(followerID uint64, vloggerID uint64) (*SocialRelation, error) {
	var relation SocialRelation
	if err := r.db.Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).First(&relation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &relation, nil
}

func (r *Repo) Create(relation *SocialRelation) error {
	return r.db.Create(relation).Error
}

func (r *Repo) DeleteByPair(followerID uint64, vloggerID uint64) (int64, error) {
	result := r.db.Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).Delete(&SocialRelation{})
	return result.RowsAffected, result.Error
}

// ListFollowerIDsAfter 按 follower_id 升序游标分页拉取粉丝 ID，供写扩散分批推送。
//
// 用游标而不是 OFFSET：扩散期间粉丝表可能正在增删，OFFSET 会让某些粉丝被跳过或重复；
// 走 (vlogger_id, follower_id) 复合索引后每一批都是一次稳定的索引区间扫描。
func (r *Repo) ListFollowerIDsAfter(vloggerID uint64, afterFollowerID uint64, limit int64) ([]uint64, error) {
	if limit <= 0 {
		return nil, nil
	}

	followerIDs := make([]uint64, 0, limit)
	if err := r.db.
		Model(&SocialRelation{}).
		Where("vlogger_id = ? AND follower_id > ?", vloggerID, afterFollowerID).
		Order("follower_id ASC").
		Limit(int(limit)).
		Pluck("follower_id", &followerIDs).Error; err != nil {
		return nil, err
	}

	return followerIDs, nil
}

func (r *Repo) FindAllFollowers(vloggerID uint64) ([]SocialRelation, error) {
	var relations []SocialRelation
	if err := r.db.
		Table("social_relations").
		Select("social_relations.*, follower.username AS follower_username").
		Joins("LEFT JOIN accounts AS follower ON follower.id = social_relations.follower_id").
		Where("social_relations.vlogger_id = ?", vloggerID).
		Order("social_relations.created_at DESC, social_relations.id DESC").
		Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *Repo) FindAllVloggers(followerID uint64) ([]SocialRelation, error) {
	var relations []SocialRelation
	if err := r.db.
		Table("social_relations").
		Select("social_relations.*, vlogger.username AS vlogger_username").
		Joins("LEFT JOIN accounts AS vlogger ON vlogger.id = social_relations.vlogger_id").
		Where("social_relations.follower_id = ?", followerID).
		Order("social_relations.created_at DESC, social_relations.id DESC").
		Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}
