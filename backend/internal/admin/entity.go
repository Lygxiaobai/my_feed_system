package admin

import "time"

// NoteMaxLength 与举报补充说明同一上限，避免处置依据把列撑爆。
const NoteMaxLength = 500

type AccessResult struct {
	Allowed bool `json:"allowed"`
}

type Overview struct {
	PendingReports int64  `json:"pending_reports"`
	AccountID      uint64 `json:"account_id"`
	Username       string `json:"username"`
}

type LookupVideoRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
}

type TakedownRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
	Note    string `json:"note"`
}

type LookupAccountRequest struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type VideoView struct {
	ID             uint64    `json:"id"`
	AuthorID       uint64    `json:"author_id"`
	Username       string    `json:"username"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	PlayURL        string    `json:"play_url"`
	CoverURL       string    `json:"cover_url"`
	LikesCount     int64     `json:"likes_count"`
	CommentCount   int64     `json:"comment_count"`
	AuditStatus    string    `json:"audit_status"`
	CreatedAt      time.Time `json:"created_at"`
	PendingReports int64     `json:"pending_reports"`
}

type AccountView struct {
	ID            uint64    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email,omitempty"`
	FollowerCount int64     `json:"follower_count"`
	CreatedAt     time.Time `json:"created_at"`
}
