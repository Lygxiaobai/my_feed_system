package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/config"
	"my_feed_system/internal/feed"
	"my_feed_system/internal/mq"
	"my_feed_system/internal/observability"
	"my_feed_system/internal/social"
)

// FanoutWorker 负责关注流的写扩散。
//
// 它消费两类事件：
//
//	video.timeline.publish  作者发布视频，写发件箱并按粉丝量级决定推送档位
//	video.fanout.batch      一批粉丝的推送任务，处理完后续投下一批
//
// 与 TimelineConsumer 消费同一个发布事件但走独立队列，两者互不影响。
type FanoutWorker struct {
	repo      *social.Repo
	db        *gorm.DB
	inbox     *feed.InboxStore
	outbox    *feed.OutboxStore
	publisher *mq.Publisher
	cfg       config.FanoutConfig
}

func NewFanoutWorker(
	db *gorm.DB,
	inbox *feed.InboxStore,
	outbox *feed.OutboxStore,
	publisher *mq.Publisher,
	cfg config.FanoutConfig,
) *FanoutWorker {
	return &FanoutWorker{
		repo:      social.NewRepo(db),
		db:        db,
		inbox:     inbox,
		outbox:    outbox,
		publisher: publisher,
		cfg:       cfg,
	}
}

func (w *FanoutWorker) Handle(ctx context.Context, event mq.Envelope) error {
	if !w.inbox.Enabled() || !w.outbox.Enabled() {
		return errors.New("following fanout stores unavailable")
	}

	switch event.EventType {
	case mq.EventTypeVideoTimelinePush:
		return w.handlePublished(ctx, event)
	case mq.EventTypeVideoFanoutBatch:
		return w.handleBatch(ctx, event)
	default:
		return fmt.Errorf("fanout worker unsupported event: %s", event.EventType)
	}
}

// handlePublished 处理一次视频发布：写作者发件箱，再决定是否发起写扩散。
func (w *FanoutWorker) handlePublished(ctx context.Context, event mq.Envelope) error {
	var payload mq.VideoTimelinePayload
	if err := event.DecodePayload(&payload); err != nil {
		return fmt.Errorf("decode video.timeline.publish payload: %w", err)
	}
	if payload.VideoID == 0 || payload.AuthorID == 0 {
		return errors.New("invalid video.timeline.publish payload for fanout")
	}

	createdAt := resolveFanoutTime(payload.CreatedAt, event.OccurredAt)

	// 无论作者是不是大V 都写发件箱：阈值可能被调整，粉丝数也会跨过阈值，
	// 发件箱始终就绪就不需要为历史内容做任何补偿。
	if err := w.outbox.Add(ctx, payload.AuthorID, payload.VideoID, createdAt); err != nil {
		return fmt.Errorf("write author outbox: %w", err)
	}

	followerCount, err := w.followerCount(payload.AuthorID)
	if err != nil {
		return fmt.Errorf("read follower count: %w", err)
	}
	if followerCount == 0 {
		return nil
	}

	mode, ok := w.resolveMode(followerCount)
	if !ok {
		// 大V 完全不推，粉丝读关注流时从发件箱实时拉取。
		slog.InfoContext(ctx, "skip following fanout for high-follower author",
			slog.Uint64("author_id", payload.AuthorID),
			slog.Int64("follower_count", followerCount))
		return nil
	}

	return w.publishBatch(ctx, mq.VideoFanoutPayload{
		VideoID:     payload.VideoID,
		AuthorID:    payload.AuthorID,
		CreatedAt:   createdAt,
		Mode:        mode,
		CursorAfter: 0,
	})
}

// handleBatch 推送一批粉丝的收件箱，并在还有剩余时投递下一批。
func (w *FanoutWorker) handleBatch(ctx context.Context, event mq.Envelope) error {
	var payload mq.VideoFanoutPayload
	if err := event.DecodePayload(&payload); err != nil {
		return fmt.Errorf("decode video.fanout.batch payload: %w", err)
	}
	if payload.VideoID == 0 || payload.AuthorID == 0 {
		return errors.New("invalid video.fanout.batch payload")
	}

	followerIDs, err := w.repo.ListFollowerIDsAfter(payload.AuthorID, payload.CursorAfter, w.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("list followers for fanout: %w", err)
	}
	if len(followerIDs) == 0 {
		return nil
	}

	targets := followerIDs
	if payload.Mode == mq.FanoutModeActive {
		targets, err = w.inbox.FilterActive(ctx, followerIDs)
		if err != nil {
			return fmt.Errorf("filter active followers: %w", err)
		}
	}

	// ZADD 幂等，因此这里不需要 processed_messages 去重：消息重投最多重复写入相同分值。
	if err := w.inbox.Push(ctx, targets, payload.VideoID, payload.CreatedAt); err != nil {
		return fmt.Errorf("push follower inboxes: %w", err)
	}
	observability.IncFeedFanoutBatch(payload.Mode, len(targets))

	if int64(len(followerIDs)) < w.cfg.BatchSize {
		return nil
	}

	// 先完成本批推送再投递下一批。反过来能让失败批次不阻断后续，
	// 但会在「已投下一批、本批未落地」时留下一个静默的空洞，
	// 而消费端失败即进死信队列，那个空洞不会有人发现。
	next := payload
	next.CursorAfter = followerIDs[len(followerIDs)-1]
	return w.publishBatch(ctx, next)
}

func (w *FanoutWorker) publishBatch(ctx context.Context, payload mq.VideoFanoutPayload) error {
	if w.publisher == nil {
		return errors.New("fanout publisher unavailable")
	}

	event, err := mq.NewEnvelope(mq.EventTypeVideoFanoutBatch, mq.ProducerWorker, payload)
	if err != nil {
		return fmt.Errorf("build video.fanout.batch event: %w", err)
	}
	return w.publisher.Publish(ctx, event)
}

// resolveMode 按粉丝数决定推送档位。ok 为 false 表示该作者不做写扩散。
func (w *FanoutWorker) resolveMode(followerCount int64) (string, bool) {
	switch {
	case followerCount < w.cfg.PushThreshold:
		return mq.FanoutModeAll, true
	case followerCount < w.cfg.PullThreshold:
		return mq.FanoutModeActive, true
	default:
		return "", false
	}
}

func (w *FanoutWorker) followerCount(authorID uint64) (int64, error) {
	counts := make([]int64, 0, 1)
	err := w.db.Model(&account.Account{}).
		Where("id = ?", authorID).
		Limit(1).
		Pluck("follower_count", &counts).Error
	if err != nil {
		return 0, err
	}
	if len(counts) == 0 {
		// 账号已注销：发件箱已经写好，不需要再扩散。
		return 0, nil
	}
	return counts[0], nil
}

func resolveFanoutTime(createdAt time.Time, occurredAt time.Time) time.Time {
	if !createdAt.IsZero() {
		return createdAt
	}
	if !occurredAt.IsZero() {
		return occurredAt
	}
	return time.Now().UTC()
}
