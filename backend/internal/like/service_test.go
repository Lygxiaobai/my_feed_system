package like

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/video"
)

func TestLikeAndUnlikeCommitRelationshipAndCounterTogether(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&video.Video{}, &VideoLike{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	item := &video.Video{
		AuthorID:    1,
		Username:    "author",
		Title:       "test",
		PlayURL:     "/static/videos/test.mp4",
		AuditStatus: audit.StatusApproved,
		Lifecycle:   video.LifecyclePublished,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	service := NewService(db, nil)
	if err := service.Like(7, LikeRequest{VideoID: item.ID}); err != nil {
		t.Fatalf("like: %v", err)
	}
	if err := service.Like(7, LikeRequest{VideoID: item.ID}); !errors.Is(err, ErrAlreadyLiked) {
		t.Fatalf("duplicate like error = %v, want ErrAlreadyLiked", err)
	}

	var liked video.Video
	if err := db.First(&liked, item.ID).Error; err != nil {
		t.Fatalf("reload liked video: %v", err)
	}
	if liked.LikesCount != 1 {
		t.Fatalf("likes_count after like = %d, want 1", liked.LikesCount)
	}

	if err := service.Unlike(7, LikeRequest{VideoID: item.ID}); err != nil {
		t.Fatalf("unlike: %v", err)
	}
	var unliked video.Video
	if err := db.First(&unliked, item.ID).Error; err != nil {
		t.Fatalf("reload unliked video: %v", err)
	}
	if unliked.LikesCount != 0 {
		t.Fatalf("likes_count after unlike = %d, want 0", unliked.LikesCount)
	}
	var relation VideoLike
	if err := db.Where("video_id = ? AND account_id = ?", item.ID, 7).First(&relation).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("like relation still exists, err=%v", err)
	}
}
