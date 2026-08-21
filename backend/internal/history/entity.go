package history

const (
	// MinPersistMs / MinPersistRatio：信息流快速滑过不落库。
	MinPersistMs    int64   = 3000
	MinPersistRatio float64 = 0.20
	// CompleteRemainMs / CompleteRatio：接近片尾视为已看完，下次从头播。
	CompleteRemainMs int64   = 2000
	CompleteRatio    float64 = 0.95
	// ResumeMinMs / ResumeMinRatio：进度太浅不 seek，避免为噪声跳转。
	ResumeMinMs    int64   = 2000
	ResumeMinRatio float64 = 0.10
	// MaxOffsetMs 拒绝离谱进度，与弹幕同一量级。
	MaxOffsetMs = 24 * 60 * 60 * 1000

	DefaultListLimit = 20
	MaxListLimit     = 50
	MaxProgressIDs   = 50

	StatusUnfinished = "unfinished"
	StatusCompleted  = "completed"
)

// WatchHistory 是登录用户对一条视频的最近一次有效观看进度。
// 每个账号每条视频只保留一行，由 upsert 覆盖。
type WatchHistory struct {
	ID         uint64 `gorm:"primaryKey" json:"id"`
	AccountID  uint64 `gorm:"not null;uniqueIndex:uk_watch_history_account_video,priority:1;index:idx_watch_history_account_done_watched,priority:1" json:"account_id"`
	VideoID    uint64 `gorm:"not null;uniqueIndex:uk_watch_history_account_video,priority:2" json:"video_id"`
	PositionMs int64  `gorm:"not null;default:0" json:"position_ms"`
	DurationMs int64  `gorm:"not null;default:0" json:"duration_ms"`
	// Completed 由服务端根据进度计算，不接受客户端布尔值。
	Completed   bool  `gorm:"not null;default:false;index:idx_watch_history_account_done_watched,priority:2" json:"completed"`
	WatchedAtMs int64 `gorm:"not null;index:idx_watch_history_account_done_watched,priority:3" json:"watched_at_ms"`
}

func (WatchHistory) TableName() string {
	return "watch_history"
}

type UpsertRequest struct {
	VideoID    uint64 `json:"video_id" binding:"required"`
	PositionMs int64  `json:"position_ms"`
	DurationMs int64  `json:"duration_ms"`
}

type ListRequest struct {
	Status string `json:"status" binding:"required"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type ProgressRequest struct {
	VideoIDs []uint64 `json:"video_ids" binding:"required"`
}

// HistoryItem 是列表里的一行：进度 + 当时仍可见的视频卡片。
type HistoryItem struct {
	VideoID       uint64 `json:"video_id"`
	PositionMs    int64  `json:"position_ms"`
	DurationMs    int64  `json:"duration_ms"`
	Completed     bool   `json:"completed"`
	ResumeMs      int64  `json:"resume_ms"`
	LastWatchedAt string `json:"last_watched_at"`
	Video         any    `json:"video"`
}

// ProgressItem 给详情页决定要不要 seek。
type ProgressItem struct {
	VideoID    uint64 `json:"video_id"`
	PositionMs int64  `json:"position_ms"`
	DurationMs int64  `json:"duration_ms"`
	Completed  bool   `json:"completed"`
	ResumeMs   int64  `json:"resume_ms"`
}
