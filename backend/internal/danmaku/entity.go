package danmaku

import "time"

const (
	// MaxContentRunes 是一条弹幕的最长字数。抖音式弹幕要短，才能在画面上扫过而不挡视频。
	MaxContentRunes = 50
	// MaxOffsetMs 拒绝离谱的播放进度，避免用超大 offset 把列表打乱。
	MaxOffsetMs = 24 * 60 * 60 * 1000
	// MaxListSize 单次列出的上限。短视频够用；超过的按进度取前一段。
	MaxListSize = 800
)

// VideoDanmaku 是一条绑在视频播放进度上的弹幕。
type VideoDanmaku struct {
	ID       uint64 `gorm:"primaryKey" json:"id"`
	VideoID  uint64 `gorm:"not null;index:idx_video_danmaku_video_offset,priority:1" json:"video_id"`
	AuthorID uint64 `gorm:"not null;index:idx_video_danmaku_author" json:"author_id"`
	Username string `gorm:"size:64;not null" json:"username"`
	Content  string `gorm:"size:200;not null" json:"content"`
	// OffsetMs 是发送时的播放进度，回放时按这个时刻飞出。
	OffsetMs  int64     `gorm:"not null;index:idx_video_danmaku_video_offset,priority:2" json:"offset_ms"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoDanmaku) TableName() string {
	return "video_danmaku"
}

type ListRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
}

type SendRequest struct {
	VideoID  uint64 `json:"video_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
	OffsetMs int64  `json:"offset_ms"`
}
