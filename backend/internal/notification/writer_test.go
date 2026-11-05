package notification

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/account"
	"my_feed_system/internal/video"
)

type relationRow struct {
	ID         uint64 `gorm:"primaryKey"`
	FollowerID uint64
	VloggerID  uint64
}

func (relationRow) TableName() string { return "social_relations" }

func setupNotifyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:notify-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Notification{}, &account.Account{}, &video.Video{}, &relationRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedAccount(t *testing.T, db *gorm.DB, id uint64, name string) {
	t.Helper()
	if err := db.Create(&account.Account{ID: id, Username: name}).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
}

func TestLikeAggregatesAndRetracts(t *testing.T) {
	db := setupNotifyDB(t)
	w := NewWriter(db)
	svc := NewService(db)

	if err := w.ApplyLike(nil, 2, 9, 1); err != nil {
		t.Fatalf("first like: %v", err)
	}
	if err := w.ApplyLike(nil, 3, 9, 1); err != nil {
		t.Fatalf("second like: %v", err)
	}
	if err := w.ApplyLike(nil, 2, 9, 1); err != nil {
		t.Fatalf("repeat like: %v", err)
	}

	listed, err := svc.List(1, ListRequest{Kind: KindLike})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("want 1 aggregated like, got %d", len(listed.Items))
	}
	if listed.Items[0].ActorCount != 2 {
		t.Fatalf("actor_count = %d, want 2", listed.Items[0].ActorCount)
	}
	if listed.Items[0].ActionText != "等 2 人赞了你的作品" {
		t.Fatalf("action = %q", listed.Items[0].ActionText)
	}

	if err := w.RetractLike(nil, 3, 9, 1); err != nil {
		t.Fatalf("retract one: %v", err)
	}
	listed, err = svc.List(1, ListRequest{Kind: KindLike})
	if err != nil {
		t.Fatalf("list after retract: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ActorCount != 1 {
		t.Fatalf("after one retract: %+v", listed.Items)
	}

	if err := w.RetractLike(nil, 2, 9, 1); err != nil {
		t.Fatalf("retract last: %v", err)
	}
	listed, err = svc.List(1, ListRequest{Kind: KindLike})
	if err != nil {
		t.Fatalf("list after hide: %v", err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("expected hidden after last unlike, got %d", len(listed.Items))
	}
}

func TestSelfActionsAreSilent(t *testing.T) {
	db := setupNotifyDB(t)
	w := NewWriter(db)
	svc := NewService(db)

	if err := w.ApplyLike(nil, 1, 9, 1); err != nil {
		t.Fatalf("self like: %v", err)
	}
	if err := w.ApplyFollow(nil, 4, 4); err != nil {
		t.Fatalf("self follow: %v", err)
	}
	if err := w.ApplyTip(nil, 8, 1, 1, 9, 10); err != nil {
		t.Fatalf("self tip: %v", err)
	}
	if err := w.ApplyComment(nil, CommentFanout{
		CommentID: 11, VideoID: 9, ActorID: 1, VideoAuthorID: 1, Content: "自己评",
	}); err != nil {
		t.Fatalf("self comment: %v", err)
	}

	listed, err := svc.List(1, ListRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("self actions should not notify, got %+v", listed.Items)
	}
}

func TestCommentReplyAndMentionSplit(t *testing.T) {
	db := setupNotifyDB(t)
	w := NewWriter(db)
	svc := NewService(db)
	seedAccount(t, db, 5, "carol")
	seedAccount(t, db, 6, "dave")

	if err := w.ApplyComment(nil, CommentFanout{
		CommentID: 21, VideoID: 9, ActorID: 2, VideoAuthorID: 1, Content: "封面好看 @carol",
	}); err != nil {
		t.Fatalf("root comment: %v", err)
	}
	if err := w.ApplyComment(nil, CommentFanout{
		CommentID:       22,
		VideoID:         9,
		ActorID:         3,
		VideoAuthorID:   1,
		ParentCommentID: 21,
		RootCommentID:   21,
		ReplyToUserID:   2,
		Content:         "同意 @carol @dave @2",
	}); err != nil {
		t.Fatalf("reply: %v", err)
	}

	author, err := svc.List(1, ListRequest{})
	if err != nil {
		t.Fatalf("author list: %v", err)
	}
	if len(author.Items) != 1 || author.Items[0].Kind != KindComment {
		t.Fatalf("author should only get the root comment, got %+v", author.Items)
	}

	replied, err := svc.List(2, ListRequest{Kind: KindReply})
	if err != nil {
		t.Fatalf("reply list: %v", err)
	}
	if len(replied.Items) != 1 || replied.Items[0].Kind != KindReply {
		t.Fatalf("comment author should get reply, got %+v", replied.Items)
	}

	mentioned, err := svc.List(5, ListRequest{Kind: KindMention})
	if err != nil {
		t.Fatalf("mention list: %v", err)
	}
	if len(mentioned.Items) != 2 {
		t.Fatalf("carol mentioned twice across comments, got %d", len(mentioned.Items))
	}

	dave, err := svc.List(6, ListRequest{Kind: KindMention})
	if err != nil {
		t.Fatalf("dave list: %v", err)
	}
	if len(dave.Items) != 1 || dave.Items[0].Kind != KindMention {
		t.Fatalf("dave should get one mention, got %+v", dave.Items)
	}

	if err := w.HideByComment(nil, 21); err != nil {
		t.Fatalf("hide: %v", err)
	}
	for _, recipient := range []uint64{1, 2, 5, 6} {
		listed, err := svc.List(recipient, ListRequest{})
		if err != nil {
			t.Fatalf("list %d: %v", recipient, err)
		}
		if len(listed.Items) != 0 {
			t.Fatalf("deleting the root should hide the subtree, recipient %d still has %+v", recipient, listed.Items)
		}
	}
}

func TestFollowAndTipAndUnread(t *testing.T) {
	db := setupNotifyDB(t)
	w := NewWriter(db)
	svc := NewService(db)
	seedAccount(t, db, 2, "bob")
	if err := db.Create(&video.Video{ID: 9, AuthorID: 1, Username: "ann", Title: "v", PlayURL: "/x", CoverURL: "/c.jpg"}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := db.Create(&relationRow{FollowerID: 1, VloggerID: 2}).Error; err != nil {
		t.Fatalf("seed follow: %v", err)
	}

	if err := w.ApplyFollow(nil, 2, 1); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if err := w.ApplyTip(nil, 77, 2, 1, 9, 20); err != nil {
		t.Fatalf("tip: %v", err)
	}

	unread, err := svc.UnreadCount(1)
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread.Count != 2 || unread.ByKind[KindFollow] != 1 || unread.ByKind[KindTip] != 1 {
		t.Fatalf("unread = %+v", unread)
	}

	listed, err := svc.List(1, ListRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(listed.Items))
	}
	if listed.Items[0].Kind != KindTip || listed.Items[0].Coins != 20 {
		t.Fatalf("latest should be tip, got %+v", listed.Items[0])
	}
	if listed.Items[1].Kind != KindFollow || listed.Items[1].Relation != RelationFollowing || !listed.Items[1].Followed {
		t.Fatalf("follow row should show 关注, got %+v", listed.Items[1])
	}
	if listed.Items[0].Video == nil || listed.Items[0].Video.CoverURL != "/c.jpg" {
		t.Fatalf("tip should carry cover, got %+v", listed.Items[0].Video)
	}

	if err := svc.MarkRead(1, []uint64{listed.Items[0].ID}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	unread, err = svc.UnreadCount(1)
	if err != nil {
		t.Fatalf("unread after mark: %v", err)
	}
	if unread.Count != 1 {
		t.Fatalf("unread after one mark = %d", unread.Count)
	}

	if err := svc.MarkAllRead(1, ""); err != nil {
		t.Fatalf("mark all: %v", err)
	}
	unread, err = svc.UnreadCount(1)
	if err != nil {
		t.Fatalf("unread after all: %v", err)
	}
	if unread.Count != 0 {
		t.Fatalf("want 0 unread, got %d", unread.Count)
	}

	other, err := svc.List(2, ListRequest{})
	if err != nil {
		t.Fatalf("other list: %v", err)
	}
	if len(other.Items) != 0 {
		t.Fatalf("must not leak recipient 1's inbox to 2")
	}
}
