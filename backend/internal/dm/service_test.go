package dm

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/account"
	"my_feed_system/internal/social"
)

func setupDMDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:dm-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &account.Account{}, &social.SocialRelation{}); err != nil {
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

func seedFollow(t *testing.T, db *gorm.DB, follower, vlogger uint64) {
	t.Helper()
	if err := db.Create(&social.SocialRelation{FollowerID: follower, VloggerID: vlogger}).Error; err != nil {
		t.Fatalf("seed follow: %v", err)
	}
}

func TestMutualFriendsCanSendMany(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	seedFollow(t, db, 1, 2)
	seedFollow(t, db, 2, 1)
	svc := NewService(db)

	for i := 0; i < 3; i++ {
		res, err := svc.Send(1, SendRequest{PeerID: 2, Content: "hi"})
		if err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
		if !res.CanSend || res.Remaining != -1 || res.Relation != RelationFriend {
			t.Fatalf("send %d meta = %+v", i, res)
		}
	}

	thread, err := svc.Thread(2, ThreadRequest{PeerID: 1})
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	if len(thread.Messages) != 3 {
		t.Fatalf("got %d messages", len(thread.Messages))
	}
}

func TestStrangerCanSendOnlyOne(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	svc := NewService(db)

	first, err := svc.Send(1, SendRequest{PeerID: 2, Content: "打招呼"})
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if first.CanSend || first.Remaining != 0 || first.Relation != RelationNone {
		t.Fatalf("first meta = %+v", first)
	}

	_, err = svc.Send(1, SendRequest{PeerID: 2, Content: "第二条"})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("second send err = %v", err)
	}

	// 对方未互关，也只能回一条。
	if _, err = svc.Send(2, SendRequest{PeerID: 1, Content: "回一句"}); err != nil {
		t.Fatalf("peer reply: %v", err)
	}
	if _, err = svc.Send(2, SendRequest{PeerID: 1, Content: "再回"}); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("peer second err = %v", err)
	}
}

func TestOneWayFollowStillCapped(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	seedFollow(t, db, 1, 2)
	svc := NewService(db)

	res, err := svc.Send(1, SendRequest{PeerID: 2, Content: "关注了你"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.Relation != RelationFollowing || res.CanSend {
		t.Fatalf("meta = %+v", res)
	}
	if _, err = svc.Send(1, SendRequest{PeerID: 2, Content: "再来"}); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("second err = %v", err)
	}
}

func TestBecomingFriendsUnlocksChat(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	svc := NewService(db)

	if _, err := svc.Send(1, SendRequest{PeerID: 2, Content: "先打个招呼"}); err != nil {
		t.Fatalf("stranger send: %v", err)
	}
	seedFollow(t, db, 1, 2)
	seedFollow(t, db, 2, 1)

	res, err := svc.Send(1, SendRequest{PeerID: 2, Content: "现在是好友"})
	if err != nil {
		t.Fatalf("friend send: %v", err)
	}
	if !res.CanSend || res.Remaining != -1 || res.Relation != RelationFriend {
		t.Fatalf("meta = %+v", res)
	}
}

func TestRejectsSelfEmptyLongAndMissing(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	svc := NewService(db)

	if _, err := svc.Send(1, SendRequest{PeerID: 1, Content: "hi"}); !errors.Is(err, ErrSelf) {
		t.Fatalf("self: %v", err)
	}
	if _, err := svc.Send(1, SendRequest{PeerID: 2, Content: "hi"}); !errors.Is(err, ErrPeerMissing) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := normalizeBody("   "); !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("empty: %v", err)
	}
	long := make([]rune, MaxBodyRunes+1)
	for i := range long {
		long[i] = '啊'
	}
	if _, err := normalizeBody(string(long)); !errors.Is(err, ErrBodyTooLong) {
		t.Fatalf("long: %v", err)
	}
}

func TestInboxUnreadAndReadReceipt(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	seedFollow(t, db, 1, 2)
	seedFollow(t, db, 2, 1)
	svc := NewService(db)

	if _, err := svc.Send(1, SendRequest{PeerID: 2, Content: "第一句"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	unread, err := svc.UnreadCount(2)
	if err != nil || unread.Count != 1 {
		t.Fatalf("unread before read = %+v err=%v", unread, err)
	}
	inbox, err := svc.Inbox(2)
	if err != nil || len(inbox.Items) != 1 || inbox.Items[0].Unread != 1 || inbox.Items[0].Preview != "第一句" {
		t.Fatalf("inbox = %+v err=%v", inbox, err)
	}

	thread, err := svc.Thread(2, ThreadRequest{PeerID: 1})
	if err != nil {
		t.Fatalf("open thread: %v", err)
	}
	if len(thread.Messages) != 1 || thread.Messages[0].Mine {
		t.Fatalf("thread = %+v", thread)
	}

	unread, err = svc.UnreadCount(2)
	if err != nil || unread.Count != 0 {
		t.Fatalf("unread after open = %+v err=%v", unread, err)
	}

	aliceView, err := svc.Thread(1, ThreadRequest{PeerID: 2})
	if err != nil {
		t.Fatalf("alice thread: %v", err)
	}
	if len(aliceView.Messages) != 1 || !aliceView.Messages[0].Mine || !aliceView.Messages[0].Read {
		t.Fatalf("read receipt = %+v", aliceView.Messages)
	}
}

func TestThreadLatestKeepsNewest(t *testing.T) {
	db := setupDMDB(t)
	seedAccount(t, db, 1, "alice")
	seedAccount(t, db, 2, "bob")
	seedFollow(t, db, 1, 2)
	seedFollow(t, db, 2, 1)
	svc := NewService(db)

	for i := 0; i < 5; i++ {
		if _, err := svc.Send(1, SendRequest{PeerID: 2, Content: "m"}); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	thread, err := svc.Thread(1, ThreadRequest{PeerID: 2, Limit: 3})
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	if !thread.HasMore || len(thread.Messages) != 3 {
		t.Fatalf("page = has_more=%v n=%d", thread.HasMore, len(thread.Messages))
	}
	// 首屏必须留下最新三条，而不是最旧三条。
	var prev uint64
	for _, msg := range thread.Messages {
		if msg.ID <= prev {
			t.Fatalf("messages not ascending newest-window: %+v", thread.Messages)
		}
		prev = msg.ID
	}
	last := thread.Messages[len(thread.Messages)-1]
	inbox, err := svc.Inbox(1)
	if err != nil || len(inbox.Items) != 1 || inbox.Items[0].LastSenderID != 1 {
		t.Fatalf("inbox %+v err=%v", inbox, err)
	}
	if last.ID != 5 {
		t.Fatalf("newest id = %d want 5", last.ID)
	}
}
