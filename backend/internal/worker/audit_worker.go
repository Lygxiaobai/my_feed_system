package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/mq"
)

// AuditWorker 消费审核事件，执行机审并在通过后放行内容。
//
// 「审核通过」这一步同时承担了原先发布时做的两件事：
// 推送全局时间线、写入热度排行榜。把它们放在这里而不是发布处，
// 是为了保证未过审内容永远不会进入任何公开数据结构。
type AuditWorker struct {
	service *audit.Service
}

func NewAuditWorker(service *audit.Service) *AuditWorker {
	return &AuditWorker{service: service}
}

func (w *AuditWorker) Handle(ctx context.Context, event mq.Envelope) error {
	if event.EventType != mq.EventTypeAuditRequested {
		return nil
	}

	var payload mq.AuditRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		// 载荷无法解析属于不可重试错误，直接进死信队列人工排查，
		// 否则会在队列里无限重投。
		return fmt.Errorf("unmarshal audit payload: %w", err)
	}
	if payload.TargetType != string(audit.TargetVideo) {
		slog.WarnContext(ctx, "unsupported audit target type", slog.String("target_type", payload.TargetType))
		return nil
	}

	// 通过后的公开化副作用由 audit.Service 在同一事务内投递，
	// 这里不重复处理——否则机审与人工两条路径会各写一份，容易只改对一边。
	if _, err := w.service.ModerateVideo(ctx, payload.TargetID); err != nil {
		if errors.Is(err, audit.ErrTargetNotFound) {
			// 内容在审核前已被删除，属于正常情况，直接 ACK 不重试。
			slog.InfoContext(ctx, "audit target already gone, skipping",
				slog.Uint64("video_id", payload.TargetID))
			return nil
		}
		return err
	}
	return nil
}
