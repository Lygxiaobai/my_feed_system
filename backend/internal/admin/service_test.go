package admin

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/account"
	"my_feed_system/internal/audit"
	"my_feed_system/internal/invoice"
	"my_feed_system/internal/report"
	"my_feed_system/internal/storage/storagetest"
	"my_feed_system/internal/tag"
	"my_feed_system/internal/video"
	"my_feed_system/internal/wallet"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&account.Account{},
		&account.Identity{},
		&video.Video{},
		&report.Report{},
		&audit.Record{},
		&invoice.Invoice{},
		&wallet.Lot{},
		&wallet.RechargeOrder{},
		&wallet.PlatformEntry{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newDummyStore 预置发布校验需要的成品对象。
func newDummyStore(t *testing.T) *storagetest.Store {
	t.Helper()
	store := storagetest.New()
	store.SeedMedia("videos/x.mp4", []byte("ftyp-placeholder"))
	store.SeedMedia("covers/x.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return store
}

func newTestService(t *testing.T, reviewerIDs []uint64) (*Service, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	videos := video.NewService(db, nil, newDummyStore(t))
	accounts := account.NewService(db, "test-jwt-secret-at-least-32-bytes-long")
	reports := report.NewService(db, videos, reviewerIDs)
	invoices := invoice.NewService(db, nil)
	wallets := wallet.NewService(db)
	return NewService(reports, videos, accounts, invoices, wallets), db
}

func seedAccount(t *testing.T, db *gorm.DB, id uint64, username, email string) {
	t.Helper()
	row := &account.Account{ID: id, Username: username, Password: "hashed"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if email != "" {
		if err := db.Create(&account.Identity{
			AccountID: id,
			Provider:  account.ProviderEmail,
			Subject:   email,
		}).Error; err != nil {
			t.Fatalf("create identity: %v", err)
		}
	}
}

func seedVideo(t *testing.T, db *gorm.DB, id, authorID uint64, username, title string, status audit.Status, tags ...string) {
	t.Helper()
	row := &video.Video{
		ID:          id,
		AuthorID:    authorID,
		Username:    username,
		Title:       title,
		Tags:        tag.List(tags),
		PlayURL:     "/static/videos/x.mp4",
		CoverURL:    "/static/covers/x.jpg",
		AuditStatus: status,
		CreatedAt:   time.Now().UTC(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
}

func TestAccessSeparatesReviewerFromOthers(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})
	seedAccount(t, db, 1, "reviewer", "1@lmr.com")
	seedAccount(t, db, 2, "viewer", "2@lmr.com")

	if !svc.Access(1).Allowed {
		t.Fatal("reviewer should be allowed")
	}
	if svc.Access(2).Allowed {
		t.Fatal("ordinary account must not enter admin")
	}
}

func TestOverviewRejectsNonReviewerAndCountsPending(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})
	seedAccount(t, db, 1, "reviewer", "")
	seedAccount(t, db, 2, "author", "")
	seedAccount(t, db, 3, "reporter", "")
	seedVideo(t, db, 10, 2, "author", "公开视频", audit.StatusApproved)

	if _, err := svc.Overview(2, "author"); err != ErrNotReviewer {
		t.Fatalf("want ErrNotReviewer, got %v", err)
	}

	if _, err := svc.reports.Submit(3, report.SubmitRequest{VideoID: 10, Reason: report.ReasonSpam}); err != nil {
		t.Fatalf("submit report: %v", err)
	}

	overview, err := svc.Overview(1, "reviewer")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.PendingReports != 1 {
		t.Fatalf("pending=%d", overview.PendingReports)
	}
}

func TestLookupVideoSeesRejectedAndPublicDoesNot(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})
	seedAccount(t, db, 1, "reviewer", "")
	seedAccount(t, db, 2, "author", "")
	seedVideo(t, db, 11, 2, "author", "已下架", audit.StatusRejected)

	view, err := svc.LookupVideo(1, LookupVideoRequest{VideoID: 11})
	if err != nil {
		t.Fatalf("admin lookup: %v", err)
	}
	if view.AuditStatus != string(audit.StatusRejected) {
		t.Fatalf("status=%s", view.AuditStatus)
	}

	_, err = svc.videos.GetDetail(1, video.GetDetailRequest{ID: 11})
	if err != video.ErrVideoNotFound {
		t.Fatalf("public detail should hide rejected, got %v", err)
	}
}

func TestTakedownRequiresNoteAndClosesReports(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})
	seedAccount(t, db, 1, "reviewer", "")
	seedAccount(t, db, 2, "author", "")
	seedAccount(t, db, 3, "reporter", "")
	seedVideo(t, db, 12, 2, "author", "待处置", audit.StatusApproved)

	if _, err := svc.reports.Submit(3, report.SubmitRequest{VideoID: 12, Reason: report.ReasonPorn, Detail: "明显违规"}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := svc.Takedown(context.Background(), 1, TakedownRequest{VideoID: 12}); err != ErrNoteRequired {
		t.Fatalf("want note required, got %v", err)
	}

	if err := svc.Takedown(context.Background(), 2, TakedownRequest{VideoID: 12, Note: "色情"}); err != ErrNotReviewer {
		t.Fatalf("want not reviewer, got %v", err)
	}

	if err := svc.Takedown(context.Background(), 1, TakedownRequest{VideoID: 12, Note: "色情内容"}); err != nil {
		t.Fatalf("takedown: %v", err)
	}

	item, err := svc.LookupVideo(1, LookupVideoRequest{VideoID: 12})
	if err != nil {
		t.Fatalf("lookup after takedown: %v", err)
	}
	if item.AuditStatus != string(audit.StatusRejected) {
		t.Fatalf("status=%s", item.AuditStatus)
	}
	if item.PendingReports != 0 {
		t.Fatalf("pending reports=%d", item.PendingReports)
	}

	_, err = svc.videos.GetDetail(3, video.GetDetailRequest{ID: 12})
	if err != video.ErrVideoNotFound {
		t.Fatalf("viewer should see missing, got %v", err)
	}

	authorView, err := svc.videos.GetDetail(2, video.GetDetailRequest{ID: 12})
	if err != nil {
		t.Fatalf("author should still see rejected: %v", err)
	}
	if authorView.AuditStatus != audit.StatusRejected {
		t.Fatalf("author status=%s", authorView.AuditStatus)
	}

	var rec audit.Record
	if err := db.Where("target_id = ?", 12).First(&rec).Error; err != nil {
		t.Fatalf("audit record: %v", err)
	}
	if rec.Moderator != "admin" {
		t.Fatalf("moderator=%s", rec.Moderator)
	}
	if rec.Detail != "色情内容" {
		t.Fatalf("detail=%s", rec.Detail)
	}
}

func TestLookupAccountByEmailDoesNotLeakPassword(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})
	seedAccount(t, db, 1, "reviewer", "")
	seedAccount(t, db, 8, "creator", "8@lmr.com")
	seedVideo(t, db, 13, 8, "creator", "作品", audit.StatusRejected)

	if _, _, err := svc.LookupAccount(8, LookupAccountRequest{ID: 8}); err != ErrNotReviewer {
		t.Fatalf("want not reviewer, got %v", err)
	}

	acc, videos, err := svc.LookupAccount(1, LookupAccountRequest{Email: "8@lmr.com"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if acc.ID != 8 || acc.Email != "8@lmr.com" || acc.Username != "creator" {
		t.Fatalf("account=%+v", acc)
	}
	if len(videos) != 1 || videos[0].AuditStatus != string(audit.StatusRejected) {
		t.Fatalf("videos=%+v", videos)
	}

	if _, _, err := svc.LookupAccount(1, LookupAccountRequest{ID: 8, Username: "creator"}); err != ErrLookupAmbiguous {
		t.Fatalf("want ambiguous, got %v", err)
	}
}
