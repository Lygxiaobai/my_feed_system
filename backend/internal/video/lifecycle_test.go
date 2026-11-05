package video

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"my_feed_system/internal/storage/storagetest"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/idempotency"
	"my_feed_system/internal/outbox"
	"my_feed_system/internal/tag"
)

func openLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &audit.Record{}, &idempotency.Key{}, &outbox.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newLifecycleStore 预置发布校验会查到的成品对象。
// 改造前这里往临时目录写文件，现在写进内存存储替身——
// 读路径已不再探测存储，只有发布校验还会 Stat 一次。
func newLifecycleStore(t *testing.T) *storagetest.Store {
	t.Helper()
	store := storagetest.New()
	store.SeedMedia("videos/x.mp4", []byte("ftyp-placeholder"))
	store.SeedMedia("covers/x.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return store
}

func newLifecycleService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := openLifecycleDB(t)
	return NewService(db, nil, newLifecycleStore(t)), db
}

func TestSaveDraftReusesPlayURLAndStaysPrivate(t *testing.T) {
	svc, _ := newLifecycleService(t)

	first, err := svc.SaveDraft(9, "author", SaveDraftRequest{
		Title:    "未写完",
		PlayURL:  "/static/videos/x.mp4",
		CoverURL: "/static/covers/x.jpg",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !first.IsDraft() || first.AuditStatus != audit.StatusPending {
		t.Fatalf("draft=%+v", first)
	}

	second, err := svc.SaveDraft(9, "author", SaveDraftRequest{
		Title:    "改过标题",
		PlayURL:  "/static/videos/x.mp4",
		CoverURL: "/static/covers/x.jpg",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same draft, got %d vs %d", second.ID, first.ID)
	}
	if second.Title != "改过标题" {
		t.Fatalf("title=%q", second.Title)
	}

	if _, err := svc.GetDetail(3, GetDetailRequest{ID: first.ID}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("stranger should not see draft, got %v", err)
	}
	own, err := svc.GetDetail(9, GetDetailRequest{ID: first.ID})
	if err != nil {
		t.Fatalf("author detail: %v", err)
	}
	if own.Title != "改过标题" {
		t.Fatalf("author title=%q", own.Title)
	}

	works, err := svc.ListByAuthorID(9, ListByAuthorIDRequest{AuthorID: 9})
	if err != nil {
		t.Fatalf("works: %v", err)
	}
	if len(works) != 0 {
		t.Fatalf("drafts must not appear in works, got %d", len(works))
	}
	drafts, err := svc.ListDrafts(9)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("drafts=%v err=%v", drafts, err)
	}
}

func TestPublishPromotesDraftAndLeavesDraftBox(t *testing.T) {
	svc, _ := newLifecycleService(t)

	draft, err := svc.SaveDraft(4, "author", SaveDraftRequest{
		PlayURL:  "/static/videos/x.mp4",
		CoverURL: "/static/covers/x.jpg",
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}

	published, err := svc.Publish(4, "author", "idem-draft-1", PublishRequest{
		Title:    "正式标题",
		PlayURL:  "/static/videos/x.mp4",
		CoverURL: "/static/covers/x.jpg",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.ID != draft.ID {
		t.Fatalf("publish should promote draft %d, got %d", draft.ID, published.ID)
	}
	if published.EffectiveLifecycle() != LifecyclePublished {
		t.Fatalf("lifecycle=%s", published.Lifecycle)
	}
	if published.AuditStatus != audit.StatusApproved {
		t.Fatalf("audit off should approve, got %s", published.AuditStatus)
	}

	drafts, err := svc.ListDrafts(4)
	if err != nil || len(drafts) != 0 {
		t.Fatalf("draft box should be empty, got %v err=%v", drafts, err)
	}

	stranger, err := svc.GetDetail(8, GetDetailRequest{ID: published.ID})
	if err != nil {
		t.Fatalf("public detail: %v", err)
	}
	if stranger.Title != "正式标题" {
		t.Fatalf("title=%q", stranger.Title)
	}
}

func TestAuthorUnpublishHidesFromPublicAndRelistRestores(t *testing.T) {
	svc, db := newLifecycleService(t)

	item := &Video{
		AuthorID:    2,
		Username:    "author",
		Title:       "在架",
		PlayURL:     "/static/videos/x.mp4",
		CoverURL:    "/static/covers/x.jpg",
		AuditStatus: audit.StatusApproved,
		Lifecycle:   LifecyclePublished,
		Tags:        tag.List{"demo"},
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.Unpublish(context.Background(), 9, AuthorVideoRequest{ID: item.ID}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("other author: %v", err)
	}

	got, err := svc.Unpublish(context.Background(), 2, AuthorVideoRequest{ID: item.ID})
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if !got.IsUnpublished() {
		t.Fatalf("lifecycle=%s", got.Lifecycle)
	}
	if got.AuditStatus != audit.StatusApproved {
		t.Fatalf("unpublish must not change audit, got %s", got.AuditStatus)
	}

	if _, err := svc.GetDetail(3, GetDetailRequest{ID: item.ID}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("public should hide unpublished, got %v", err)
	}
	if _, err := svc.GetDetail(2, GetDetailRequest{ID: item.ID}); err != nil {
		t.Fatalf("author should still see: %v", err)
	}

	works, err := svc.ListByAuthorID(2, ListByAuthorIDRequest{AuthorID: 2})
	if err != nil || len(works) != 1 || works[0].Lifecycle != LifecycleUnpublished {
		t.Fatalf("author works=%v err=%v", works, err)
	}
	others, err := svc.ListByAuthorID(3, ListByAuthorIDRequest{AuthorID: 2})
	if err != nil || len(others) != 0 {
		t.Fatalf("stranger works=%v err=%v", others, err)
	}

	relisted, err := svc.Relist(context.Background(), 2, AuthorVideoRequest{ID: item.ID})
	if err != nil {
		t.Fatalf("relist: %v", err)
	}
	if !relisted.IsPubliclyListed() {
		t.Fatalf("relisted=%+v", relisted)
	}
	if _, err := svc.GetDetail(3, GetDetailRequest{ID: item.ID}); err != nil {
		t.Fatalf("public after relist: %v", err)
	}
}

func TestDeleteHidesFromEveryoneIncludingAuthor(t *testing.T) {
	svc, db := newLifecycleService(t)

	item := &Video{
		AuthorID:    5,
		Username:    "author",
		Title:       "删掉",
		PlayURL:     "/static/videos/x.mp4",
		CoverURL:    "/static/covers/x.jpg",
		AuditStatus: audit.StatusApproved,
		Lifecycle:   LifecyclePublished,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Delete(context.Background(), 5, AuthorVideoRequest{ID: item.ID}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetDetail(5, GetDetailRequest{ID: item.ID}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("author should not see deleted, got %v", err)
	}
	if _, err := svc.GetDetail(1, GetDetailRequest{ID: item.ID}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("public should not see deleted, got %v", err)
	}

	var row Video
	if err := db.First(&row, item.ID).Error; err != nil {
		t.Fatalf("row must remain: %v", err)
	}
	if row.DeletedAt == nil {
		t.Fatal("expected soft delete")
	}

	works, err := svc.ListByAuthorID(5, ListByAuthorIDRequest{AuthorID: 5})
	if err != nil || len(works) != 0 {
		t.Fatalf("deleted must leave works, got %v err=%v", works, err)
	}
}

func TestScopePublicIgnoresDraftAndUnpublished(t *testing.T) {
	approved := &Video{AuditStatus: audit.StatusApproved, Lifecycle: LifecyclePublished}
	if !approved.IsPubliclyListed() {
		t.Fatal("published+approved should be public")
	}
	draft := &Video{AuditStatus: audit.StatusPending, Lifecycle: LifecycleDraft}
	if draft.IsPubliclyListed() {
		t.Fatal("draft must not be public")
	}
	hidden := &Video{AuditStatus: audit.StatusApproved, Lifecycle: LifecycleUnpublished}
	if hidden.IsPubliclyListed() {
		t.Fatal("unpublished must not be public")
	}
	if visibleTo(hidden, 0) {
		t.Fatal("anonymous must not see unpublished")
	}
	hidden.AuthorID = 7
	if !visibleTo(hidden, 7) {
		t.Fatal("author should see unpublished")
	}
}
