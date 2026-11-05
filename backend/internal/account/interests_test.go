package account

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRecordFromVideoPersistsJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:account_interests?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Account{Username: "u", Password: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, "test-jwt-secret-at-least-32-bytes-long")
	if err := svc.RecordFromVideo(1, []string{"美食"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordFromVideo(1, []string{"旅行"}); err != nil {
		t.Fatal(err)
	}
	var row Account
	if err := db.Select("interests").First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	got := []string(row.Interests)
	if !reflect.DeepEqual(got, []string{"美食", "旅行"}) {
		t.Fatalf("stored %v parsed %v", row.Interests, got)
	}
	raw := EncodeInterests(row.Interests)
	if !strings.Contains(raw, `"美食"`) {
		t.Fatalf("raw json %s", raw)
	}
}

func TestRecordFromVideoAcceptsNullInterests(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:account_interests_null?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO accounts (id, username, password, follower_count, interests) VALUES (1, 'u', 'x', 0, NULL)").Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, "test-jwt-secret-at-least-32-bytes-long")
	if err := svc.RecordFromVideo(1, []string{"美食"}); err != nil {
		t.Fatal(err)
	}
	var row Account
	if err := db.Select("interests").First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string(row.Interests), []string{"美食"}) {
		t.Fatalf("got %v", row.Interests)
	}
}
