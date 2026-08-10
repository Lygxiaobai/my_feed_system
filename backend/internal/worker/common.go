package worker

import (
	"context"
	"log/slog"
	"time"

	"my_feed_system/internal/mq"
	"my_feed_system/internal/observability"
	"my_feed_system/internal/video"
)

func invalidateVideoDetailCache(cache *video.DetailCache, publisher *mq.Publisher, videoID uint64) {
	if cache == nil && publisher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if cache != nil {
		if err := cache.Delete(ctx, videoID); err != nil {
			slog.WarnContext(ctx, "invalidate video detail cache failed", slog.Uint64("video_id", videoID), slog.String("error", err.Error()))
		} else {
			observability.IncCacheInvalidation(observability.CacheVideoDetail, "l2", "write")
		}
	}
	if publisher != nil {
		if err := publisher.PublishCacheInvalidated(ctx, mq.CacheInvalidatedPayload{
			Cache:   mq.CacheNameVideoDetail,
			VideoID: videoID,
		}); err != nil {
			slog.WarnContext(ctx, "publish detail invalidation failed", slog.Uint64("video_id", videoID), slog.String("error", err.Error()))
		}
	}
}

func publishPopularityChanged(ctx context.Context, publisher *mq.Publisher, payload mq.PopularityChangedPayload) {
	// 热度事件属于副作用，尽量不影响主写链路。
	if publisher == nil {
		return
	}

	env, err := mq.NewEnvelope(mq.EventTypePopularityChanged, mq.ProducerWorker, payload)
	if err != nil {
		slog.ErrorContext(ctx, "build popularity event failed", slog.String("error", err.Error()))
		return
	}
	if err := publisher.Publish(ctx, env); err != nil {
		slog.ErrorContext(ctx, "publish popularity event failed",
			slog.Uint64("video_id", payload.VideoID), slog.Int64("delta", int64(payload.Delta)), slog.String("error", err.Error()))
	}
}
