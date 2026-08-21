package mq

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	MessageVersionV1 = 1

	// ProducerAPIServer 表示事件由 API 请求路径产生。
	ProducerAPIServer = "api-server"
	// ProducerWorker 表示事件由 Worker 消费后继续派生产生。
	ProducerWorker = "worker"
)

const (
	EventTypeLikeCreated             = "like.created"
	EventTypeLikeDeleted             = "like.deleted"
	EventTypeCommentCreated          = "comment.created"
	EventTypeCommentDeleted          = "comment.deleted"
	EventTypeSocialFollowed          = "social.followed"
	EventTypeSocialUnfollowed        = "social.unfollowed"
	EventTypePopularityChanged       = "popularity.changed"
	EventTypeVideoTimelinePush       = "video.timeline.publish"
	EventTypeVideoFanoutBatch        = "video.fanout.batch"
	EventTypeCacheInvalidated        = "cache.invalidated"
	EventTypeMediaTranscodeRequested = "media.transcode.requested"
	EventTypeAuditRequested          = "audit.requested"
	EventTypeTipCreated              = "tip.created"
	EventTypeVideoEmbedRequested     = "video.embed.requested"
)

const (
	CacheNameVideoDetail = "video.detail"
	CacheNameFeedLatest  = "feed.latest"
)

type Envelope struct {
	// EventID 全局唯一，用于消费端幂等去重。
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Producer   string    `json:"producer"`
	// Version 预留给后续消息结构升级。
	Version int             `json:"version"`
	Payload json.RawMessage `json:"payload"`
}

func NewEnvelope(eventType string, producer string, payload any) (Envelope, error) {
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}

	return Envelope{
		EventID:    newEventID(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		Producer:   producer,
		Version:    MessageVersionV1,
		Payload:    payloadRaw,
	}, nil
}

func (e Envelope) DecodePayload(dst any) error {
	return json.Unmarshal(e.Payload, dst)
}

func newEventID() string {
	// 时间戳 + 随机串，兼顾可排序性与低碰撞概率。
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}

type LikePayload struct {
	AccountID uint64 `json:"account_id"`
	VideoID   uint64 `json:"video_id"`
}

type MediaTranscodePayload struct {
	TaskID    uint64 `json:"task_id"`
	AccountID uint64 `json:"account_id"`
}

// AuditRequestedPayload 触发一次内容审核。
// 只带 ID 不带内容本身：内容以数据库为准，避免事件重投时用到过期快照。
type AuditRequestedPayload struct {
	TargetType string `json:"target_type"`
	TargetID   uint64 `json:"target_id"`
	AuthorID   uint64 `json:"author_id"`
}

type CommentCreatedPayload struct {
	CommentID       uint64 `json:"comment_id"`
	VideoID         uint64 `json:"video_id"`
	AuthorID        uint64 `json:"author_id"`
	Username        string `json:"username"`
	Content         string `json:"content"`
	ParentCommentID uint64 `json:"parent_comment_id"`
	RootCommentID   uint64 `json:"root_comment_id"`
	ReplyToUserID   uint64 `json:"reply_to_user_id"`
	ReplyToUsername string `json:"reply_to_username"`
}

type CommentDeletedPayload struct {
	CommentID  uint64 `json:"comment_id"`
	VideoID    uint64 `json:"video_id"`
	OperatorID uint64 `json:"operator_id"`
}

type SocialPayload struct {
	FollowerID uint64 `json:"follower_id"`
	VloggerID  uint64 `json:"vlogger_id"`
}

type PopularityChangedPayload struct {
	VideoID uint64 `json:"video_id"`
	Delta   int64  `json:"delta"`
	Reason  string `json:"reason"`
}

type VideoTimelinePayload struct {
	VideoID   uint64    `json:"video_id"`
	AuthorID  uint64    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

// 关注流写扩散的推送档位。
const (
	// FanoutModeAll 推给作者的全部粉丝，用于普通作者。
	FanoutModeAll = "all"
	// FanoutModeActive 只推给活跃粉丝，用于粉丝量中等的作者。
	// 冷粉丝不推，他们下次打开关注流时会自行回源重建收件箱。
	FanoutModeActive = "active"
)

// VideoFanoutPayload 描述一批写扩散任务。
//
// 粉丝按 follower_id 升序游标分批，每条消息只处理一批，处理完再投递下一批。
// 不在单条消息里推完的原因很实在：消费端对单条消息有 10 秒处理上限，
// 且失败直接进死信队列不重试，大V 一次推完必然超时并丢掉整次扩散。
type VideoFanoutPayload struct {
	VideoID   uint64    `json:"video_id"`
	AuthorID  uint64    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
	Mode      string    `json:"mode"`
	// CursorAfter 是上一批处理到的 follower_id，本批从它之后继续。
	CursorAfter uint64 `json:"cursor_after"`
}

type CacheInvalidatedPayload struct {
	Cache   string `json:"cache"`
	VideoID uint64 `json:"video_id,omitempty"`
	Version int64  `json:"version,omitempty"`
}

// VideoEmbedPayload 触发对一条已公开视频的标题/描述向量计算。
type VideoEmbedPayload struct {
	VideoID uint64 `json:"video_id"`
}
