package recommend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/account"
	"my_feed_system/internal/audit"
	"my_feed_system/internal/config"
	"my_feed_system/internal/like"
	"my_feed_system/internal/popularity"
	"my_feed_system/internal/tag"
	"my_feed_system/internal/video"
	"my_feed_system/internal/wallet"
)

func openRecDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&account.Account{},
		&video.Video{},
		&like.VideoLike{},
		&wallet.TipRecord{},
		&VideoEmbedding{},
		&UserEmbedding{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedAccount(t *testing.T, db *gorm.DB, id uint64, followers int64) {
	t.Helper()
	if err := db.Create(&account.Account{
		ID:            id,
		Username:      fmt.Sprintf("user-%d", id),
		Password:      "x",
		FollowerCount: followers,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedVideo(t *testing.T, db *gorm.DB, id, author uint64, status audit.Status, title string, created time.Time, tags ...string) {
	t.Helper()
	if err := db.Create(&video.Video{
		ID:          id,
		AuthorID:    author,
		Username:    "u",
		Title:       title,
		Tags:        tag.List(tags),
		PlayURL:     "/static/videos/x.mp4",
		AuditStatus: status,
		CreatedAt:   created,
		Popularity:  int64(id),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestListExcludesUnapproved(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 1, 0)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	seedVideo(t, db, 1, 2, audit.StatusApproved, "ok", now)
	seedVideo(t, db, 2, 2, audit.StatusPending, "no", now)
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 1 || res.Videos[0].ID != 1 {
		t.Fatalf("videos=%v", res.Videos)
	}
}

func TestListKeepsPersistedHotWhenRedisWindowIsThin(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	for i := uint64(1); i <= 3; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "v", now.Add(-time.Duration(i)*time.Minute))
	}

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	popularityService := popularity.NewService(redisClient)
	if err := popularityService.Record(context.Background(), 1, 999, now); err != nil {
		t.Fatal(err)
	}

	svc := NewService(db, nil, redisClient, popularityService, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 0, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 3 {
		t.Fatalf("expected MySQL candidates, got %d", len(res.Videos))
	}
	if res.Scores[1] != 1 {
		t.Fatalf("thin window should not overlay Redis score, got %d", res.Scores[1])
	}
}

func TestListOverlaysRedisHotWhenSnapshotIsUsable(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	for i := uint64(1); i <= 10; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "v", now.Add(-time.Duration(i)*time.Minute))
	}

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	popularityService := popularity.NewService(redisClient)
	for i := uint64(1); i <= 10; i++ {
		if err := popularityService.Record(context.Background(), i, float64(1000-i), now); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(db, nil, redisClient, popularityService, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 0, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) == 0 {
		t.Fatal("expected recommend videos")
	}
	for _, item := range res.Videos {
		if res.Scores[item.ID] < 990 {
			t.Fatalf("usable snapshot should overlay Redis score for %d, got %d", item.ID, res.Scores[item.ID])
		}
	}
}

func TestListAnonymousColdStart(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	for i := uint64(1); i <= 3; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "v", now.Add(-time.Duration(i)*time.Minute))
	}
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 0, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) == 0 {
		t.Fatal("anonymous recommend should not be empty")
	}
}

func TestListHidesOwnVideos(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 1, 0)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	seedVideo(t, db, 1, 1, audit.StatusApproved, "mine", now)
	seedVideo(t, db, 2, 2, audit.StatusApproved, "theirs", now)
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Videos {
		if item.AuthorID == 1 {
			t.Fatal("own video leaked into recommend")
		}
	}
}

type stubEmbedder struct{ model string }

func (s stubEmbedder) Enabled() bool { return false }
func (s stubEmbedder) Model() string { return s.model }
func (s stubEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, nil
}

func TestListUsesStoredTagsAndFillsToTen(t *testing.T) {
	db := openRecDB(t)
	if err := db.Create(&account.Account{
		ID: 1, Username: "viewer", Password: "x",
		Interests: tag.List{"美食"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedAccount(t, db, 2, 0)
	now := time.Now()
	seedVideo(t, db, 1, 2, audit.StatusApproved, "探店", now, "美食")
	for i := uint64(2); i <= 12; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "普通视频", now.Add(-time.Duration(i)*time.Minute))
	}
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 10 {
		t.Fatalf("want 10 videos, got %d", len(res.Videos))
	}
	if res.Videos[0].ID != 1 || res.Videos[0].Title != "探店" {
		t.Fatalf("tag match should come first: %+v", res.Videos[0])
	}
}

func TestListDoesNotMatchTitleAsTag(t *testing.T) {
	db := openRecDB(t)
	if err := db.Create(&account.Account{
		ID: 1, Username: "viewer", Password: "x",
		Interests: tag.List{"美食"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedAccount(t, db, 2, 0)
	now := time.Now()
	seedVideo(t, db, 1, 2, audit.StatusApproved, "#美食 探店", now)
	seedVideo(t, db, 2, 2, audit.StatusApproved, "夜市", now.Add(-time.Minute), "夜市")
	seedVideo(t, db, 3, 2, audit.StatusApproved, "探店", now.Add(-2*time.Minute), "美食")
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) == 0 || res.Videos[0].ID != 3 {
		t.Fatalf("stored tag should win over a title hashtag: %+v", res.Videos)
	}
}

func TestListDefaultWhenNoTags(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 1, 0)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	for i := uint64(1); i <= 4; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "v", now.Add(-time.Duration(i)*time.Minute))
	}
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 4 {
		t.Fatalf("default fill should return inventory, got %d", len(res.Videos))
	}
}

func TestListDefaultWhenInterestsNull(t *testing.T) {
	db := openRecDB(t)
	if err := db.Exec("INSERT INTO accounts (id, username, password, follower_count, interests) VALUES (1, 'viewer', 'x', 0, NULL)").Error; err != nil {
		t.Fatal(err)
	}
	seedAccount(t, db, 2, 0)
	now := time.Now()
	for i := uint64(1); i <= 4; i++ {
		seedVideo(t, db, i, 2, audit.StatusApproved, "v", now.Add(-time.Duration(i)*time.Minute))
	}
	svc := NewService(db, nil, nil, nil, nil, config.RecommendConfig{})
	res, err := svc.List(context.Background(), 1, ListRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 4 {
		t.Fatalf("null interests should use default mix, got %d", len(res.Videos))
	}
}

func TestInvalidateUserPersistsInterest(t *testing.T) {
	db := openRecDB(t)
	seedAccount(t, db, 1, 0)
	seedAccount(t, db, 2, 0)
	now := time.Now()
	seedVideo(t, db, 10, 2, audit.StatusApproved, "liked", now)
	vec := normalize([]float32{1, 0})
	if err := db.Create(&VideoEmbedding{
		VideoID:   10,
		Model:     "unit",
		Dim:       2,
		Vector:    EncodeVector(vec),
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&like.VideoLike{VideoID: 10, AccountID: 1, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewService(db, stubEmbedder{model: "unit"}, nil, nil, nil, config.RecommendConfig{})
	svc.InvalidateUser(1)

	row, err := NewRepo(db).FindUserEmbedding(1)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.Model != "unit" || row.Dim != 2 || len(row.Vector) == 0 {
		t.Fatalf("expected persisted user embedding, got %+v", row)
	}

	if err := db.Where("account_id = ?", 1).Delete(&like.VideoLike{}).Error; err != nil {
		t.Fatal(err)
	}
	svc.InvalidateUser(1)
	row, err = NewRepo(db).FindUserEmbedding(1)
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatal("user embedding should be removed when interest signals are gone")
	}
}
