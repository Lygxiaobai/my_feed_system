package notification

import "time"

// Kind 是互动通知的类型。列表筛选与写入共用同一组取值。
type Kind string

const (
	KindFollow  Kind = "follow"
	KindLike    Kind = "like"
	KindComment Kind = "comment"
	KindReply   Kind = "reply"
	KindMention Kind = "mention"
	KindTip     Kind = "tip"
)

// FilterKind 是列表筛选值。reply 同时覆盖评论与回复，对应产品上的「评论」。
func (k Kind) ValidFilter() bool {
	switch k {
	case "", "all", KindFollow, KindLike, KindComment, KindReply, KindMention, KindTip:
		return true
	}
	return false
}

const (
	RelationFriend    = "friend"
	RelationFollowing = "following"
)

const (
	maxStoredActors  = 20
	maxShownActors   = 3
	textClipRunes    = 80
	maxMentions      = 10
	defaultListLimit = 20
	maxListLimit     = 50
)

// Notification 是投递给某个接收者的一条互动通知。
//
// 点赞按作品聚合：同一视频只保留一行，后来的点赞更新演员列表并重新标未读。
// 关注 / 评论 / @ / 打赏各写各的，不聚合——钱和对话被卷进「等 N 人」会丢语义。
type Notification struct {
	ID          uint64 `gorm:"primaryKey"`
	RecipientID uint64 `gorm:"not null;uniqueIndex:uk_notifications_recipient_dedup,priority:1;index:idx_notifications_inbox,priority:1"`
	Kind        Kind   `gorm:"size:16;not null;index:idx_notifications_inbox_kind,priority:2"`

	ActorID    uint64 `gorm:"not null"`
	ActorIDs   string `gorm:"size:512;not null;default:''"`
	ActorCount int    `gorm:"not null;default:1"`

	VideoID       uint64 `gorm:"not null;default:0"`
	CommentID     uint64 `gorm:"not null;default:0"`
	RootCommentID uint64 `gorm:"not null;default:0"`
	TipID         uint64 `gorm:"not null;default:0"`
	Coins         int64  `gorm:"not null;default:0"`
	Text          string `gorm:"size:240;not null;default:''"`

	DedupKey string `gorm:"size:80;not null;uniqueIndex:uk_notifications_recipient_dedup,priority:2"`
	Hidden   bool   `gorm:"not null;default:0;index:idx_notifications_inbox,priority:2;index:idx_notifications_inbox_kind,priority:3"`
	ReadAt   *time.Time

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null;index:idx_notifications_inbox,priority:3;index:idx_notifications_inbox_kind,priority:4"`
}

func (Notification) TableName() string { return "notifications" }

type ListRequest struct {
	Kind   Kind   `json:"kind"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type MarkReadRequest struct {
	IDs []uint64 `json:"ids"`
}

type MarkAllReadRequest struct {
	Kind Kind `json:"kind"`
}

type ActorView struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

type VideoPreview struct {
	ID       uint64 `json:"id"`
	CoverURL string `json:"cover_url"`
	Title    string `json:"title"`
}

type Item struct {
	ID         uint64        `json:"id"`
	Kind       Kind          `json:"kind"`
	Actors     []ActorView   `json:"actors"`
	ActorCount int           `json:"actor_count"`
	Text       string        `json:"text"`
	ActionText string        `json:"action_text"`
	Relation   string        `json:"relation"`
	Followed   bool          `json:"followed"`
	Video      *VideoPreview `json:"video"`
	Coins      int64         `json:"coins"`
	Unread     bool          `json:"unread"`
	CreatedAt  time.Time     `json:"created_at"`
}

type ListResult struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type UnreadCount struct {
	Count  int64          `json:"count"`
	ByKind map[Kind]int64 `json:"by_kind"`
}

// CommentFanout 是评论落库后用来生成通知的最小输入。
// 不引用 mq 包，避免通知模块和消息层绑死。
type CommentFanout struct {
	CommentID       uint64
	VideoID         uint64
	ActorID         uint64
	VideoAuthorID   uint64
	ParentCommentID uint64
	RootCommentID   uint64
	ReplyToUserID   uint64
	Content         string
}
