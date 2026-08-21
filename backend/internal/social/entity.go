package social

import "time"

// SocialRelation 表示用户之间的关注关系。
//
// idx_social_relations_vlogger_follower 是写扩散专用的复合索引：
// 扩散按 `vlogger_id = ? AND follower_id > ?` 游标分批拉取粉丝，
// 只有 vlogger_id 的单列索引会在粉丝多时退化成排序 + 回表。
type SocialRelation struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	FollowerID       uint64    `gorm:"not null;uniqueIndex:uk_social_relations_pair;index:idx_social_relations_vlogger_follower,priority:2" json:"follower_id"`
	VloggerID        uint64    `gorm:"not null;uniqueIndex:uk_social_relations_pair;index:idx_social_relations_vlogger;index:idx_social_relations_vlogger_follower,priority:1" json:"vlogger_id"`
	CreatedAt        time.Time `json:"created_at"`
	FollowerUsername string    `gorm:"->;-:migration" json:"follower_username,omitempty"`
	VloggerUsername  string    `gorm:"->;-:migration" json:"vlogger_username,omitempty"`
}

// TableName 指定关注关系表名。
func (SocialRelation) TableName() string {
	return "social_relations"
}

// FollowRequest 用于关注和取关。
type FollowRequest struct {
	VloggerID uint64 `json:"vlogger_id" binding:"required"`
}

// GetAllFollowersRequest 查询某个作者的全部粉丝。
type GetAllFollowersRequest struct {
	VloggerID uint64 `json:"vlogger_id"`
}

// GetAllVloggersRequest 查询某个用户关注的作者列表。
type GetAllVloggersRequest struct {
	FollowerID uint64 `json:"follower_id"`
}
