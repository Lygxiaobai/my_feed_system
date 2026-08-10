package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/redis/go-redis/v9"

	"my_feed_system/internal/config"
	"my_feed_system/internal/db"
	"my_feed_system/internal/feed"
	"my_feed_system/internal/logging"
	"my_feed_system/internal/mq"
	"my_feed_system/internal/observability"
	"my_feed_system/internal/popularity"
	"my_feed_system/internal/video"
	workerpkg "my_feed_system/internal/worker"
)

// fatal 记录致命错误后退出，理由同 API 侧：启动期依赖不可用应交由编排层重启。
func fatal(msg string, err error) {
	slog.Error(msg, slog.String("error", err.Error()))
	os.Exit(1)
}

func main() {
	// 先用环境变量把日志跑起来，保证「配置加载失败」本身也有结构化日志可查。
	logging.Setup(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"), "worker")

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fatal("load config failed", err)
	}
	logging.Setup(
		firstNonEmpty(os.Getenv("LOG_LEVEL"), cfg.Log.Level),
		firstNonEmpty(os.Getenv("LOG_FORMAT"), cfg.Log.Format),
		"worker",
	)

	database, err := db.NewMySQL(cfg.Database)
	if err != nil {
		fatal("connect mysql failed", err)
	}

	var redisClient *redis.Client
	redisClient, err = db.NewRedis(cfg.Redis)
	if err != nil {
		slog.Warn("redis unavailable, popularity updates will be skipped", slog.String("error", err.Error()))
	} else {
		defer func() {
			if closeErr := redisClient.Close(); closeErr != nil {
				slog.Warn("close redis failed", slog.String("error", closeErr.Error()))
			}
		}()
	}

	var popularityService *popularity.Service
	if redisClient != nil {
		popularityService = popularity.NewService(redisClient)
	}

	var redisCmd redis.Cmdable
	if redisClient != nil {
		redisCmd = redisClient
	}

	detailCache := video.NewDetailCache(redisCmd)
	latestCache := feed.NewLatestCache(redisCmd)
	timelineStore := feed.NewGlobalTimelineStore(redisCmd)

	rabbitConn, err := mq.Dial(cfg.RabbitMQ)
	if err != nil {
		fatal("connect rabbitmq failed", err)
	}
	defer func() {
		if closeErr := rabbitConn.Close(); closeErr != nil {
			slog.Warn("close rabbitmq failed", slog.String("error", closeErr.Error()))
		}
	}()

	if err := mq.DeclareTopology(rabbitConn); err != nil {
		fatal("declare rabbitmq topology failed", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := observability.StartPprof(ctx, "worker", cfg.Pprof.Worker); err != nil {
		fatal("start worker pprof failed", err)
	}

	publisher := mq.NewPublisher(rabbitConn)
	likeWorker := workerpkg.NewLikeWorker(database, publisher, detailCache)
	commentWorker := workerpkg.NewCommentWorker(database, publisher, detailCache)
	socialWorker := workerpkg.NewSocialWorker(database)
	popularityWorker := workerpkg.NewPopularityWorker(database, popularityService, detailCache)
	timelineConsumer := workerpkg.NewTimelineConsumer(timelineStore, latestCache, publisher)
	mediaWorker := workerpkg.NewMediaWorker(database, cfg.Upload.Dir)
	popularityProjectionPoller := popularity.NewProjectionPoller(popularity.NewProjectionRepo(database), popularityService)

	consumerTagPrefix := strings.TrimSpace(cfg.RabbitMQ.ConsumerTag)
	if consumerTagPrefix == "" {
		consumerTagPrefix = "feed-worker"
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 16)

	start := func(queue string, suffix string, handler mq.HandlerFunc) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			tag := fmt.Sprintf("%s-%s", consumerTagPrefix, suffix)
			consumer := mq.NewConsumer(rabbitConn, queue, tag, cfg.RabbitMQ.PrefetchCount, handler)
			slog.Info("consumer started", slog.String("queue", queue), slog.String("tag", tag))
			if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("run consumer queue=%s: %w", queue, err)
			}
		}()
	}

	start(mq.QueueLikeWrite, "like", likeWorker.Handle)
	start(mq.QueueCommentWrite, "comment", commentWorker.Handle)
	start(mq.QueueSocialWrite, "social", socialWorker.Handle)
	start(mq.QueuePopularityUpdate, "popularity", popularityWorker.Handle)
	start(mq.QueueTimelineUpdate, "timeline", timelineConsumer.Handle)
	start(mq.QueueMediaTranscode, "media", mediaWorker.Handle)

	for _, spec := range mq.QueueSpecs() {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()

			tag := fmt.Sprintf("%s-dlq-%s", consumerTagPrefix, spec.Queue)
			consumer := mq.NewDeadLetterConsumer(rabbitConn, database, spec.DLQ, tag, cfg.RabbitMQ.PrefetchCount)
			slog.Info("dlq consumer started",
				slog.String("queue", spec.DLQ), slog.String("source_queue", spec.Queue), slog.String("tag", tag))
			if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("run dlq consumer queue=%s: %w", spec.DLQ, err)
			}
		}()
	}

	if popularityService != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.Info("popularity projection poller started")
			popularityProjectionPoller.Run(ctx)
		}()
	}

	select {
	case <-ctx.Done():
		slog.Info("worker shutting down")
	case runErr := <-errCh:
		slog.Error("worker stopped by consumer error", slog.String("error", runErr.Error()))
		stop()
	}

	wg.Wait()
	slog.Info("worker exited")
}

// firstNonEmpty 返回第一个非空字符串，用于实现「环境变量优先于配置文件」。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
