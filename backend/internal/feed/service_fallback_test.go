package feed

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/popularity"
	"my_feed_system/internal/video"
)

func TestListByPopularityFallsBackToMySQLWhenRedisIsEmpty(t *testing.T) {
	db := newFeedFallbackTestDB(t, 1)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	service := NewService(db, popularity.NewService(redisClient))
	result, err := service.ListByPopularity(ListByPopularityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list popularity feed failed: %v", err)
	}
	if len(result.Videos) != 1 {
		t.Fatalf("expected one fallback video, got %d", len(result.Videos))
	}
	if result.Videos[0].Popularity != 1 {
		t.Fatalf("expected persisted popularity 1, got %d", result.Videos[0].Popularity)
	}
	if result.AsOf != 0 {
		t.Fatalf("expected fallback as_of 0, got %d", result.AsOf)
	}
}

func TestListByPopularityFallsBackWhenRedisBelowTen(t *testing.T) {
	db := newFeedFallbackTestDB(t, 15)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	popularityService := popularity.NewService(redisClient)
	now := time.Now()
	for id := uint64(1); id <= 3; id++ {
		if err := popularityService.Record(context.Background(), id, float64(1000-id), now); err != nil {
			t.Fatalf("record popularity failed: %v", err)
		}
	}

	service := NewService(db, popularityService)
	result, err := service.ListByPopularity(ListByPopularityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list popularity feed failed: %v", err)
	}
	if len(result.Videos) != 10 {
		t.Fatalf("expected a full MySQL page, got %d", len(result.Videos))
	}
	if result.Videos[0].ID != 15 || result.Videos[0].Popularity != 15 {
		t.Fatalf("expected MySQL order to start at id=15, got id=%d popularity=%d", result.Videos[0].ID, result.Videos[0].Popularity)
	}
	if result.AsOf != 0 {
		t.Fatalf("expected fallback as_of 0, got %d", result.AsOf)
	}
}

func TestListByPopularityStaysOnMySQLWhenPagingFallback(t *testing.T) {
	db := newFeedFallbackTestDB(t, 15)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	popularityService := popularity.NewService(redisClient)
	now := time.Now()
	for id := uint64(1); id <= 3; id++ {
		if err := popularityService.Record(context.Background(), id, float64(1000-id), now); err != nil {
			t.Fatalf("record popularity failed: %v", err)
		}
	}

	service := NewService(db, popularityService)
	first, err := service.ListByPopularity(ListByPopularityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("first page failed: %v", err)
	}
	if first.AsOf != 0 {
		t.Fatalf("expected first page as_of 0, got %d", first.AsOf)
	}

	for id := uint64(1); id <= 15; id++ {
		if err := popularityService.Record(context.Background(), id, float64(1000-id), now); err != nil {
			t.Fatalf("record later popularity failed: %v", err)
		}
	}

	second, err := service.ListByPopularity(ListByPopularityRequest{
		Limit:  10,
		AsOf:   0,
		Offset: first.NextOffset,
	})
	if err != nil {
		t.Fatalf("second page failed: %v", err)
	}
	if second.AsOf != 0 {
		t.Fatalf("expected sticky MySQL as_of 0, got %d", second.AsOf)
	}
	if len(second.Videos) != 5 || second.Videos[0].ID != 5 {
		t.Fatalf("expected MySQL page two to start at id=5, got %+v", second.Videos)
	}
}

func TestListByPopularityFallsBackWhenFirstPageCannotFill(t *testing.T) {
	db := newFeedFallbackTestDB(t, 12)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	popularityService := popularity.NewService(redisClient)
	now := time.Now()
	// 快照成员数够 10，但都是库里没有的脏 ID，拼装后首屏为空。
	for id := uint64(100); id < 110; id++ {
		if err := popularityService.Record(context.Background(), id, float64(id), now); err != nil {
			t.Fatalf("record stale popularity failed: %v", err)
		}
	}

	service := NewService(db, popularityService)
	result, err := service.ListByPopularity(ListByPopularityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list popularity feed failed: %v", err)
	}
	if result.AsOf != 0 {
		t.Fatalf("expected fallback as_of 0, got %d", result.AsOf)
	}
	if len(result.Videos) != 10 || result.Videos[0].ID != 12 {
		t.Fatalf("expected MySQL catalog page, got %+v", result.Videos)
	}
}

func TestListByPopularityPrefersRedisWhenRankingExists(t *testing.T) {
	db := newFeedFallbackTestDB(t, 10)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	popularityService := popularity.NewService(redisClient)
	now := time.Now()
	for id := uint64(1); id <= 10; id++ {
		if err := popularityService.Record(context.Background(), id, float64(1000-id), now); err != nil {
			t.Fatalf("record popularity failed: %v", err)
		}
	}

	service := NewService(db, popularityService)
	result, err := service.ListByPopularity(ListByPopularityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list popularity feed failed: %v", err)
	}
	if len(result.Videos) != 10 {
		t.Fatalf("expected a full Redis page, got %d", len(result.Videos))
	}
	if result.Videos[0].ID != 1 || result.Videos[0].Popularity != 999 {
		t.Fatalf("expected Redis order to start at id=1 score=999, got id=%d popularity=%d", result.Videos[0].ID, result.Videos[0].Popularity)
	}
	if result.AsOf == 0 {
		t.Fatal("expected Redis snapshot as_of")
	}
}

func newFeedFallbackTestDB(t *testing.T, n int) *gorm.DB {
	t.Helper()
	if n <= 0 {
		t.Fatal("need at least one test video")
	}

	testDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(testDir, "feed-test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(&video.Video{}); err != nil {
		t.Fatalf("migrate video table: %v", err)
	}

	now := time.Now()
	for id := 1; id <= n; id++ {
		record := &video.Video{
			ID:          uint64(id),
			AuthorID:    1,
			Username:    "test",
			Title:       "fallback",
			PlayURL:     "/static/videos/" + strconv.Itoa(id) + ".mp4",
			AuditStatus: audit.StatusApproved,
			Popularity:  int64(id),
			CreatedAt:   now.Add(-time.Duration(id) * time.Second),
			UpdatedAt:   now,
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create test video: %v", err)
		}
	}

	return db
}

// Redis 整体不可用时，最新流必须退回 MySQL 持久化热度出数，而不是整页 500（QA BUG-005）。
func TestListLatestFallsBackToPersistedPopularityWhenRedisIsDown(t *testing.T) {
	db := newFeedFallbackTestDB(t, 3)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	redisServer.Close()

	service := NewService(db, popularity.NewService(redisClient))
	result, err := service.ListLatest(ListLatestRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list latest must not fail when redis is down: %v", err)
	}
	if len(result.Videos) != 3 {
		t.Fatalf("expected 3 videos from MySQL, got %d", len(result.Videos))
	}
	for _, item := range result.Videos {
		if item.Popularity != int64(item.ID) {
			t.Fatalf("expected persisted popularity %d for video %d, got %d", item.ID, item.ID, item.Popularity)
		}
	}
}
