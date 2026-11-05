package account

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 改名后作品、评论、弹幕里冗余的用户名必须同步，否则 Feed 卡片和评论区仍显示旧名（QA BUG-004）。
func TestRenameSyncsDenormalizedUsernames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:account_rename?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE videos (id INTEGER PRIMARY KEY, author_id INTEGER, username TEXT)`,
		`CREATE TABLE video_comments (id INTEGER PRIMARY KEY, author_id INTEGER, username TEXT, reply_to_user_id INTEGER, reply_to_username TEXT)`,
		`CREATE TABLE video_danmaku (id INTEGER PRIMARY KEY, author_id INTEGER, username TEXT)`,
		`INSERT INTO videos VALUES (1, 1, 'old'), (2, 2, 'other')`,
		`INSERT INTO video_comments VALUES (1, 1, 'old', 0, ''), (2, 2, 'other', 1, 'old')`,
		`INSERT INTO video_danmaku VALUES (1, 1, 'old'), (2, 2, 'other')`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&Account{Username: "old", Password: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Account{Username: "other", Password: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewService(db, "test-jwt-secret-at-least-32-bytes-long")
	if _, err := svc.Rename(1, RenameRequest{NewUsername: "new"}); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		query string
		want  string
	}{
		{`SELECT username FROM videos WHERE id = 1`, "new"},
		{`SELECT username FROM videos WHERE id = 2`, "other"},
		{`SELECT username FROM video_comments WHERE id = 1`, "new"},
		{`SELECT reply_to_username FROM video_comments WHERE id = 2`, "new"},
		{`SELECT username FROM video_comments WHERE id = 2`, "other"},
		{`SELECT username FROM video_danmaku WHERE id = 1`, "new"},
		{`SELECT username FROM video_danmaku WHERE id = 2`, "other"},
	}
	for _, c := range checks {
		var got string
		if err := db.Raw(c.query).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.query, got, c.want)
		}
	}
}
