package worker

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/config"
	"my_feed_system/internal/feed"
	"my_feed_system/internal/mq"
	"my_feed_system/internal/social"
)

func TestFanoutWorkerResolvesModeByFollowerCount(t *testing.T) {
	worker := &FanoutWorker{cfg: config.FanoutConfig{PushThreshold: 100, PullThreshold: 1000}}

	cases := []struct {
		name          string
		followerCount int64
		wantMode      string
		wantFanout    bool
	}{
		{"普通作者全量推", 99, mq.FanoutModeAll, true},
		{"中V 只推活跃粉丝", 100, mq.FanoutModeActive, true},
		{"大V 完全不推", 1000, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, ok := worker.resolveMode(tc.followerCount)
			if ok != tc.wantFanout || mode != tc.wantMode {
				t.Fatalf("follower_count=%d expected (%q,%v), got (%q,%v)",
					tc.followerCount, tc.wantMode, tc.wantFanout, mode, ok)
			}
		})
	}
}

func TestFanoutWorkerActiveModeSkipsInactiveFollowers(t *testing.T) {
	db := newFanoutTestDB(t)
	inbox, outbox, _ := newFanoutTestStores(t)
	ctx := context.Background()

	// 三个粉丝，只有 2 号读过关注流因而带活跃标记。
	for _, followerID := range []uint64{2, 3, 4} {
		if err := db.Create(&social.SocialRelation{FollowerID: followerID, VloggerID: 10}).Error; err != nil {
			t.Fatalf("create relation: %v", err)
		}
	}
	if err := inbox.MarkActive(ctx, 2); err != nil {
		t.Fatalf("mark active: %v", err)
	}

	// BatchSize 大于粉丝数，本批处理完即结束，不会再投递下一批。
	worker := NewFanoutWorker(db, inbox, outbox, nil, config.FanoutConfig{
		PushThreshold: 100,
		PullThreshold: 1000,
		BatchSize:     10,
	})

	event, err := mq.NewEnvelope(mq.EventTypeVideoFanoutBatch, mq.ProducerWorker, mq.VideoFanoutPayload{
		VideoID:   777,
		AuthorID:  10,
		CreatedAt: time.Now(),
		Mode:      mq.FanoutModeActive,
	})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if err := worker.Handle(ctx, event); err != nil {
		t.Fatalf("handle fanout batch failed: %v", err)
	}

	assertInboxHas(t, inbox, 2, 777, true)
	assertInboxHas(t, inbox, 3, 777, false)
	assertInboxHas(t, inbox, 4, 777, false)
}

func TestFanoutWorkerAllModePushesEveryFollower(t *testing.T) {
	db := newFanoutTestDB(t)
	inbox, outbox, _ := newFanoutTestStores(t)
	ctx := context.Background()

	for _, followerID := range []uint64{2, 3, 4} {
		if err := db.Create(&social.SocialRelation{FollowerID: followerID, VloggerID: 10}).Error; err != nil {
			t.Fatalf("create relation: %v", err)
		}
	}

	worker := NewFanoutWorker(db, inbox, outbox, nil, config.FanoutConfig{
		PushThreshold: 100,
		PullThreshold: 1000,
		BatchSize:     10,
	})

	event, err := mq.NewEnvelope(mq.EventTypeVideoFanoutBatch, mq.ProducerWorker, mq.VideoFanoutPayload{
		VideoID:   888,
		AuthorID:  10,
		CreatedAt: time.Now(),
		Mode:      mq.FanoutModeAll,
	})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	// 重复消费一次，验证 ZADD 幂等下扩散可以安全重投。
	for i := 0; i < 2; i++ {
		if err := worker.Handle(ctx, event); err != nil {
			t.Fatalf("handle fanout batch failed: %v", err)
		}
	}

	for _, followerID := range []uint64{2, 3, 4} {
		assertInboxHas(t, inbox, followerID, 888, true)
		ids, _, err := inbox.ListVideoIDs(ctx, followerID, 0, 100)
		if err != nil {
			t.Fatalf("read inbox: %v", err)
		}
		if len(ids) != 1 {
			t.Fatalf("expected redelivery to stay idempotent, follower %d got %v", followerID, ids)
		}
	}
}

func TestSocialWorkerMaintainsFollowerCount(t *testing.T) {
	db := newFanoutTestDB(t)
	inbox, _, following := newFanoutTestStores(t)
	ctx := context.Background()

	if err := db.Create(&account.Account{ID: 10, Username: "vlogger"}).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	worker := NewSocialWorkerWithFanout(db, inbox, following)
	payload := mq.SocialPayload{FollowerID: 2, VloggerID: 10}

	followEvent := newSocialEvent(t, mq.EventTypeSocialFollowed, payload)
	if err := worker.Handle(ctx, followEvent); err != nil {
		t.Fatalf("handle follow failed: %v", err)
	}
	assertFollowerCount(t, db, 10, 1)

	// 同一条消息重投：幂等闸门挡住，计数不能再加。
	if err := worker.Handle(ctx, followEvent); err != nil {
		t.Fatalf("handle duplicated follow failed: %v", err)
	}
	assertFollowerCount(t, db, 10, 1)

	// 换一个 event_id 的重复关注：靠唯一索引挡住，计数同样不能再加。
	if err := worker.Handle(ctx, newSocialEvent(t, mq.EventTypeSocialFollowed, payload)); err != nil {
		t.Fatalf("handle repeated follow failed: %v", err)
	}
	assertFollowerCount(t, db, 10, 1)

	if err := worker.Handle(ctx, newSocialEvent(t, mq.EventTypeSocialUnfollowed, payload)); err != nil {
		t.Fatalf("handle unfollow failed: %v", err)
	}
	assertFollowerCount(t, db, 10, 0)

	// 关系已不存在时再取关，计数不能被带成负数。
	if err := worker.Handle(ctx, newSocialEvent(t, mq.EventTypeSocialUnfollowed, payload)); err != nil {
		t.Fatalf("handle repeated unfollow failed: %v", err)
	}
	assertFollowerCount(t, db, 10, 0)
}

func TestSocialWorkerClearsFollowingFeedState(t *testing.T) {
	db := newFanoutTestDB(t)
	inbox, _, following := newFanoutTestStores(t)
	ctx := context.Background()

	if err := db.Create(&account.Account{ID: 10, Username: "vlogger"}).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := inbox.MarkActive(ctx, 2); err != nil {
		t.Fatalf("mark active: %v", err)
	}
	if err := following.Set(ctx, 2, []feed.FollowedAuthor{{VloggerID: 99}}); err != nil {
		t.Fatalf("seed following cache: %v", err)
	}

	worker := NewSocialWorkerWithFanout(db, inbox, following)
	err := worker.Handle(ctx, newSocialEvent(t, mq.EventTypeSocialFollowed, mq.SocialPayload{FollowerID: 2, VloggerID: 10}))
	if err != nil {
		t.Fatalf("handle follow failed: %v", err)
	}

	// 关注列表变了，收件箱相对新关系已不完整，必须触发下一次读取时重建。
	active, err := inbox.IsActive(ctx, 2)
	if err != nil {
		t.Fatalf("read active flag: %v", err)
	}
	if active {
		t.Fatal("expected inbox active flag cleared after follow")
	}
	if _, ok, err := following.Get(ctx, 2); err != nil || ok {
		t.Fatalf("expected following cache invalidated, ok=%v err=%v", ok, err)
	}
}

func newSocialEvent(t *testing.T, eventType string, payload mq.SocialPayload) mq.Envelope {
	t.Helper()

	event, err := mq.NewEnvelope(eventType, mq.ProducerAPIServer, payload)
	if err != nil {
		t.Fatalf("build social event: %v", err)
	}
	return event
}

func assertFollowerCount(t *testing.T, db *gorm.DB, accountID uint64, want int64) {
	t.Helper()

	var record account.Account
	if err := db.First(&record, accountID).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	if record.FollowerCount != want {
		t.Fatalf("expected follower_count %d, got %d", want, record.FollowerCount)
	}
}

func assertInboxHas(t *testing.T, inbox *feed.InboxStore, userID uint64, videoID uint64, want bool) {
	t.Helper()

	ids, _, err := inbox.ListVideoIDs(context.Background(), userID, 0, 100)
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == videoID {
			found = true
			break
		}
	}
	if found != want {
		t.Fatalf("inbox of %d: expected contains(%d)=%v, got %v", userID, videoID, want, found)
	}
}

func newFanoutTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "fanout-test.db")), &gorm.Config{})
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
	if err := db.AutoMigrate(&social.SocialRelation{}, &account.Account{}, &mq.ProcessedMessage{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	return db
}

func newFanoutTestStores(t *testing.T) (*feed.InboxStore, *feed.OutboxStore, *feed.FollowingCache) {
	t.Helper()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	return feed.NewInboxStore(redisClient, 1000, time.Hour),
		feed.NewOutboxStore(redisClient, 200),
		feed.NewFollowingCache(redisClient, time.Minute)
}
