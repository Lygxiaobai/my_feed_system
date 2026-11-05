package video

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"my_feed_system/internal/storage/storagetest"

	"my_feed_system/internal/audit"
)

func newTakedownTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:takedown?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &audit.Record{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM videos")
		db.Exec("DELETE FROM audit_records")
	})

	return NewService(db, nil, storagetest.New()), db
}

func TestTakedownRejectsVideoAndRecordsTrail(t *testing.T) {
	service, db := newTakedownTestService(t)

	item := &Video{
		AuthorID:    1,
		Username:    "author",
		Title:       "违规内容",
		PlayURL:     "/static/videos/a.mp4",
		AuditStatus: audit.StatusApproved,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	if err := service.Takedown(context.Background(), item.ID, 8, "举报属实"); err != nil {
		t.Fatalf("takedown: %v", err)
	}

	var reloaded Video
	if err := db.First(&reloaded, item.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.AuditStatus != audit.StatusRejected {
		t.Fatalf("audit_status = %q, want %q", reloaded.AuditStatus, audit.StatusRejected)
	}

	// 流水必须留下：处置记录的留存期是合规要求，日志按容量轮转顶不住。
	var records []audit.Record
	if err := db.Where("target_id = ?", item.ID).Find(&records).Error; err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}

	got := records[0]
	if got.FromStatus != audit.StatusApproved || got.ToStatus != audit.StatusRejected {
		t.Fatalf("transition = %q -> %q, want approved -> rejected", got.FromStatus, got.ToStatus)
	}
	if got.Source != audit.SourceManual {
		t.Fatalf("source = %q, want %q", got.Source, audit.SourceManual)
	}
	if got.OperatorID != 8 {
		t.Fatalf("operator_id = %d, want 8", got.OperatorID)
	}
	if got.Moderator != takedownModerator {
		t.Fatalf("moderator = %q, want %q so report takedowns are distinguishable from machine review", got.Moderator, takedownModerator)
	}
}

func TestTakedownMissingVideo(t *testing.T) {
	service, _ := newTakedownTestService(t)

	if err := service.Takedown(context.Background(), 12345, 8, ""); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("got %v, want ErrVideoNotFound", err)
	}
}

// 下架后内容对非作者不可见，对作者仍可见（他需要知道自己的内容被处置了）。
func TestTakenDownVideoIsHiddenFromOthers(t *testing.T) {
	service, db := newTakedownTestService(t)

	item := &Video{
		AuthorID:    1,
		Username:    "author",
		Title:       "违规内容",
		PlayURL:     "/static/videos/a.mp4",
		AuditStatus: audit.StatusApproved,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	if err := service.Takedown(context.Background(), item.ID, 8, ""); err != nil {
		t.Fatalf("takedown: %v", err)
	}

	var reloaded Video
	if err := db.First(&reloaded, item.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if visibleTo(&reloaded, 2) {
		t.Fatal("taken-down video is still visible to a non-author viewer")
	}
	if !visibleTo(&reloaded, 1) {
		t.Fatal("taken-down video is hidden from its own author")
	}
}
