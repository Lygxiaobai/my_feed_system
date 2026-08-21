package dm

import "time"

const (
	// MaxBodyRunes 是一条私信的最长字数。先做纯文本互聊，图文卡片以后再加。
	MaxBodyRunes = 500
	// StrangerQuota 未互关时，每名发送者对同一对象只能发出这么多条。
	StrangerQuota = 1
	// PreviewRunes 会话列表里的摘要长度。
	PreviewRunes     = 40
	defaultListLimit = 50
	maxListLimit     = 100
	maxInboxSize     = 50
)

const (
	RelationFriend    = "friend"
	RelationFollowing = "following"
	RelationFollower  = "follower"
	RelationNone      = "none"
)

// Conversation 是两个账号之间的唯一会话。
//
// user_lo < user_hi，避免 A→B 和 B→A 各建一行。未读和已读游标按两侧分开存，
// 这样列表和「已读」都不必扫消息表。
type Conversation struct {
	ID            uint64     `gorm:"primaryKey"`
	UserLo        uint64     `gorm:"not null;uniqueIndex:uk_dm_conversations_pair,priority:1"`
	UserHi        uint64     `gorm:"not null;uniqueIndex:uk_dm_conversations_pair,priority:2"`
	LastMessageID uint64     `gorm:"not null;default:0"`
	LastPreview   string     `gorm:"size:160;not null;default:''"`
	LastSenderID  uint64     `gorm:"not null;default:0"`
	LastAt        time.Time  `gorm:"not null;index:idx_dm_conversations_last_at"`
	LoUnread      int        `gorm:"not null;default:0"`
	HiUnread      int        `gorm:"not null;default:0"`
	LoLastReadAt  *time.Time `gorm:""`
	HiLastReadAt  *time.Time `gorm:""`
	CreatedAt     time.Time  `gorm:"not null"`
}

func (Conversation) TableName() string { return "dm_conversations" }

func (c Conversation) PeerID(me uint64) uint64 {
	if me == c.UserLo {
		return c.UserHi
	}
	return c.UserLo
}

func (c Conversation) UnreadOf(me uint64) int {
	if me == c.UserLo {
		return c.LoUnread
	}
	return c.HiUnread
}

func (c Conversation) LastReadAt(me uint64) *time.Time {
	if me == c.UserLo {
		return c.LoLastReadAt
	}
	return c.HiLastReadAt
}

// Message 是会话里的一条文本私信。只追加，本轮不做撤回或编辑。
type Message struct {
	ID             uint64    `gorm:"primaryKey;index:idx_dm_messages_conv,priority:2"`
	ConversationID uint64    `gorm:"not null;index:idx_dm_messages_conv,priority:1"`
	SenderID       uint64    `gorm:"not null"`
	Body           string    `gorm:"size:1500;not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (Message) TableName() string { return "dm_messages" }

type InboxRequest struct{}

type ThreadRequest struct {
	PeerID   uint64 `json:"peer_id" binding:"required"`
	AfterID  uint64 `json:"after_id"`
	BeforeID uint64 `json:"before_id"`
	Limit    int    `json:"limit"`
}

type SendRequest struct {
	PeerID  uint64 `json:"peer_id" binding:"required"`
	Content string `json:"content"`
}

type MarkReadRequest struct {
	PeerID uint64 `json:"peer_id" binding:"required"`
}

type PeerView struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

type ConversationView struct {
	Peer         PeerView  `json:"peer"`
	Preview      string    `json:"preview"`
	Unread       int       `json:"unread"`
	LastAt       time.Time `json:"last_at"`
	LastSenderID uint64    `json:"last_sender_id"`
}

type MessageView struct {
	ID        uint64    `json:"id"`
	SenderID  uint64    `json:"sender_id"`
	Body      string    `json:"body"`
	Mine      bool      `json:"mine"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type InboxResult struct {
	Items []ConversationView `json:"items"`
}

type ThreadResult struct {
	Peer      PeerView      `json:"peer"`
	Relation  string        `json:"relation"`
	CanSend   bool          `json:"can_send"`
	Remaining int           `json:"remaining"`
	Messages  []MessageView `json:"messages"`
	HasMore   bool          `json:"has_more"`
}

type SendResult struct {
	Message   MessageView `json:"message"`
	Relation  string      `json:"relation"`
	CanSend   bool        `json:"can_send"`
	Remaining int         `json:"remaining"`
}

type UnreadCount struct {
	Count int64 `json:"count"`
}
