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

	"my_feed_system/internal/account"
	"my_feed_system/internal/audit"
	"my_feed_system/internal/social"
	"my_feed_system/internal/video"
)

// 测试固定的角色：
//
//	1   读关注流的用户，关注了 10 和 20
//	10  普通作者，视频走写扩散进 1 的收件箱
//	20  大V，视频不推，只留在自己的发件箱里
//	30  未被关注的作者，用于验证取关残留会被过滤
const (
	testReaderID = uint64(1)
	testNormalID = uint64(10)
	testBigVID   = uint64(20)
	testStranger = uint64(30)

	testNormalVideoID   = uint64(100)
	testBigVVideoID     = uint64(200)
	testStrangerVideoID = uint64(300)
)

func TestFollowingFeedMergesInboxAndBigVOutbox(t *testing.T) {
	env := newFollowingTestEnv(t, 1000, 20)

	env.markActive(t, testReaderID)
	env.pushInbox(t, testReaderID, testNormalVideoID)
	env.addOutbox(t, testBigVID, testBigVVideoID)

	result, err := env.service.ListByFollowing(testReaderID, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}

	// 大V 的视频从未进过收件箱，只可能来自发件箱的读扩散。
	if got := videoIDsOf(result.Videos); len(got) != 2 || got[0] != testBigVVideoID || got[1] != testNormalVideoID {
		t.Fatalf("expected merged feed [%d %d], got %v", testBigVVideoID, testNormalVideoID, got)
	}
	if result.HasMore {
		t.Fatal("expected no more pages")
	}
}

func TestFollowingFeedFiltersUnfollowedResidue(t *testing.T) {
	env := newFollowingTestEnv(t, 1000, 20)

	env.markActive(t, testReaderID)
	env.pushInbox(t, testReaderID, testNormalVideoID)
	// 模拟取关后仍残留在收件箱里的视频。
	env.pushInbox(t, testReaderID, testStrangerVideoID)

	result, err := env.service.ListByFollowing(testReaderID, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}

	if got := videoIDsOf(result.Videos); len(got) != 1 || got[0] != testNormalVideoID {
		t.Fatalf("expected only followed author video, got %v", got)
	}

	remaining, _, err := env.inbox.ListVideoIDs(context.Background(), testReaderID, 0, 100)
	if err != nil {
		t.Fatalf("read inbox failed: %v", err)
	}
	for _, videoID := range remaining {
		if videoID == testStrangerVideoID {
			t.Fatal("expected unfollowed residue to be removed from inbox")
		}
	}
}

func TestFollowingFeedRebuildsInboxWhenNotActive(t *testing.T) {
	env := newFollowingTestEnv(t, 1000, 20)

	// 不打活跃标记、收件箱为空：冷启动、被淘汰、长期不活跃都会落到这条路径。
	result, err := env.service.ListByFollowing(testReaderID, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}
	if got := videoIDsOf(result.Videos); len(got) != 2 {
		t.Fatalf("expected rebuilt feed to contain both followed videos, got %v", got)
	}

	active, err := env.inbox.IsActive(context.Background(), testReaderID)
	if err != nil {
		t.Fatalf("read active flag failed: %v", err)
	}
	if !active {
		t.Fatal("expected inbox to be marked active after rebuild")
	}

	rebuilt, _, err := env.inbox.ListVideoIDs(context.Background(), testReaderID, 0, 100)
	if err != nil {
		t.Fatalf("read inbox failed: %v", err)
	}
	if len(rebuilt) != 2 {
		t.Fatalf("expected inbox rebuilt with 2 entries, got %v", rebuilt)
	}
}

func TestFollowingFeedFallsBackWhenInboxTruncated(t *testing.T) {
	// 收件箱容量压到 1，凑不满一页时不能断言「没有下一页」。
	env := newFollowingTestEnv(t, 1, 20)

	env.markActive(t, testReaderID)
	env.pushInbox(t, testReaderID, testNormalVideoID)

	result, err := env.service.ListByFollowing(testReaderID, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}

	// 大V 的视频既不在收件箱也不在发件箱，只有回落 MySQL 才拿得到。
	if got := videoIDsOf(result.Videos); len(got) != 2 {
		t.Fatalf("expected MySQL fallback to return both videos, got %v", got)
	}
}

func TestFollowingFeedFallsBackWhenTooManyBigV(t *testing.T) {
	// 允许合并的大V 数量为 0，等于任何大V 都触发降级。
	env := newFollowingTestEnv(t, 1000, 0)

	env.markActive(t, testReaderID)
	env.pushInbox(t, testReaderID, testNormalVideoID)

	result, err := env.service.ListByFollowing(testReaderID, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}
	if got := videoIDsOf(result.Videos); len(got) != 2 {
		t.Fatalf("expected MySQL fallback to return both videos, got %v", got)
	}
}

func TestFollowingFeedReturnsEmptyWithoutQueryingWhenNothingFollowed(t *testing.T) {
	env := newFollowingTestEnv(t, 1000, 20)

	result, err := env.service.ListByFollowing(testStranger, ListByFollowingRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list following feed failed: %v", err)
	}
	if len(result.Videos) != 0 {
		t.Fatalf("expected empty feed, got %v", videoIDsOf(result.Videos))
	}
	if result.HasMore {
		t.Fatal("expected no more pages")
	}
}

type followingTestEnv struct {
	service *Service
	inbox   *InboxStore
	outbox  *OutboxStore
	baseAt  time.Time
}

func (e *followingTestEnv) markActive(t *testing.T, userID uint64) {
	t.Helper()
	if err := e.inbox.MarkActive(context.Background(), userID); err != nil {
		t.Fatalf("mark inbox active failed: %v", err)
	}
}

func (e *followingTestEnv) pushInbox(t *testing.T, userID uint64, videoID uint64) {
	t.Helper()
	if err := e.inbox.Push(context.Background(), []uint64{userID}, videoID, e.createdAt(videoID)); err != nil {
		t.Fatalf("push inbox failed: %v", err)
	}
}

func (e *followingTestEnv) addOutbox(t *testing.T, authorID uint64, videoID uint64) {
	t.Helper()
	if err := e.outbox.Add(context.Background(), authorID, videoID, e.createdAt(videoID)); err != nil {
		t.Fatalf("add outbox failed: %v", err)
	}
}

// createdAt 让视频 ID 越大发布时间越晚，便于断言排序结果。
func (e *followingTestEnv) createdAt(videoID uint64) time.Time {
	return e.baseAt.Add(time.Duration(videoID) * time.Second)
}

func newFollowingTestEnv(t *testing.T, inboxMaxSize int64, maxPullAuthors int) *followingTestEnv {
	t.Helper()

	baseAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	db := newFollowingTestDB(t, baseAt)

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	inbox := NewInboxStore(redisClient, inboxMaxSize, time.Hour)
	outbox := NewOutboxStore(redisClient, 200)
	service := NewService(db, nil).WithFollowingFanout(&FollowingFanout{
		Inbox:     inbox,
		Outbox:    outbox,
		Following: NewFollowingCache(redisClient, time.Minute),
		// 阈值取 1000：普通作者 1 个粉丝走写扩散，大V 50 万粉丝走读扩散。
		PullThreshold:  1000,
		MaxPullAuthors: maxPullAuthors,
	})

	return &followingTestEnv{service: service, inbox: inbox, outbox: outbox, baseAt: baseAt}
}

func newFollowingTestDB(t *testing.T, baseAt time.Time) *gorm.DB {
	t.Helper()

	testDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(testDir, "following-test.db")), &gorm.Config{})
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
	if err := db.AutoMigrate(&video.Video{}, &social.SocialRelation{}, &account.Account{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	accounts := []account.Account{
		{ID: testReaderID, Username: "reader"},
		{ID: testNormalID, Username: "normal", FollowerCount: 1},
		{ID: testBigVID, Username: "bigv", FollowerCount: 500000},
		{ID: testStranger, Username: "stranger"},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatalf("create account: %v", err)
		}
	}

	relations := []social.SocialRelation{
		{FollowerID: testReaderID, VloggerID: testNormalID},
		{FollowerID: testReaderID, VloggerID: testBigVID},
	}
	for i := range relations {
		if err := db.Create(&relations[i]).Error; err != nil {
			t.Fatalf("create relation: %v", err)
		}
	}

	videos := []struct {
		id       uint64
		authorID uint64
	}{
		{testNormalVideoID, testNormalID},
		{testBigVVideoID, testBigVID},
		{testStrangerVideoID, testStranger},
	}
	for _, item := range videos {
		record := &video.Video{
			ID:          item.id,
			AuthorID:    item.authorID,
			Username:    "author",
			Title:       "t",
			PlayURL:     "/static/videos/" + videoFileName(item.id),
			AuditStatus: audit.StatusApproved,
			CreatedAt:   baseAt.Add(time.Duration(item.id) * time.Second),
			UpdatedAt:   baseAt,
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create test video: %v", err)
		}
	}

	return db
}

func videoFileName(videoID uint64) string {
	return strconv.FormatUint(videoID, 10) + ".mp4"
}

func videoIDsOf(videos []FeedVideo) []uint64 {
	ids := make([]uint64, 0, len(videos))
	for _, item := range videos {
		ids = append(ids, item.ID)
	}
	return ids
}
