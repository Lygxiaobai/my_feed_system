package admin

import (
	"errors"
	"testing"
	"time"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/like"
	"my_feed_system/internal/wallet"
)

func TestCatalogListsVideosAccountsAndInterestTags(t *testing.T) {
	svc, db := newTestService(t, []uint64{1})

	seedAccount(t, db, 1, "reviewer", "1@lmr.com")
	seedAccount(t, db, 2, "author", "2@lmr.com")
	seedAccount(t, db, 3, "viewer", "")
	seedVideo(t, db, 21, 2, "author", "公开作品", audit.StatusApproved)
	seedVideo(t, db, 22, 2, "author", "探店", audit.StatusRejected, "美食")

	if err := db.AutoMigrate(&like.VideoLike{}, &wallet.TipRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&like.VideoLike{AccountID: 3, VideoID: 22, CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.accounts.RecordFromVideo(3, []string{"美食"}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListVideos(3, ListVideosRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("ordinary list videos: %v", err)
	}
	if _, err := svc.ListAccounts(3, ListAccountsRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("ordinary list accounts: %v", err)
	}

	videos, err := svc.ListVideos(1, ListVideosRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if videos.Summary.Total != 2 || videos.Summary.Approved != 1 || videos.Summary.Rejected != 1 {
		t.Fatalf("video summary %+v", videos.Summary)
	}
	if len(videos.Videos) != 2 {
		t.Fatalf("video rows=%d", len(videos.Videos))
	}
	seenRejected := false
	for _, item := range videos.Videos {
		if item.ID == 22 && item.AuditStatus == string(audit.StatusRejected) {
			seenRejected = true
			if len(item.Tags) != 1 || item.Tags[0] != "美食" {
				t.Fatalf("rejected video tags %+v", item.Tags)
			}
		}
	}
	if !seenRejected {
		t.Fatalf("rejected video missing: %+v", videos.Videos)
	}

	if _, err := svc.ListVideos(1, ListVideosRequest{AuditStatus: "alipay"}); !errors.Is(err, ErrInvalidAuditStatus) {
		t.Fatalf("invalid status: %v", err)
	}

	accounts, err := svc.ListAccounts(1, ListAccountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if accounts.Summary.Total != 3 || len(accounts.Accounts) != 3 {
		t.Fatalf("account board %+v rows=%d", accounts.Summary, len(accounts.Accounts))
	}
	var viewer *AccountView
	for i := range accounts.Accounts {
		row := &accounts.Accounts[i]
		if row.ID == 3 {
			viewer = row
		}
	}
	if viewer == nil || len(viewer.InterestTags) == 0 || viewer.InterestTags[0].Label != "美食" {
		t.Fatalf("viewer tags %+v", viewer)
	}

	acc, works, err := svc.LookupAccount(1, LookupAccountRequest{ID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if acc.Username != "viewer" || len(acc.InterestTags) == 0 || acc.InterestTags[0].Label != "美食" {
		t.Fatalf("lookup tags %+v", acc)
	}
	if len(works) != 0 {
		t.Fatalf("viewer should have no works, got %d", len(works))
	}

	overview, err := svc.Overview(1, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if overview.VideoCount != 2 || overview.AccountCount != 3 {
		t.Fatalf("overview counts videos=%d accounts=%d", overview.VideoCount, overview.AccountCount)
	}

}
