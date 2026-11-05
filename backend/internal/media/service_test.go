package media

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"my_feed_system/internal/outbox"
	"my_feed_system/internal/storage"
	"my_feed_system/internal/storage/storagetest"
)

func newMediaService(t *testing.T) (*Service, *storagetest.Store) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Task{}, &outbox.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM media_tasks")
		db.Exec("DELETE FROM outbox_messages")
	})
	store := storagetest.New()
	return NewService(db, store, 64<<20), store
}

func TestInitMultipartSignsEveryPart(t *testing.T) {
	svc, _ := newMediaService(t)

	// 20MiB / 8MiB = 3 段
	result, err := svc.InitMultipart(context.Background(), 7, 20<<20, UploadPartBytes)
	if err != nil {
		t.Fatalf("init multipart: %v", err)
	}
	if result.PartCount != 3 {
		t.Fatalf("part count = %d, want 3", result.PartCount)
	}
	if len(result.PartURLs) != result.PartCount {
		t.Fatalf("got %d urls for %d parts", len(result.PartURLs), result.PartCount)
	}
	// 对象键必须带上账号段，拼装时才能校验归属。
	if !strings.HasPrefix(result.ObjectKey, "sources/7/") {
		t.Fatalf("object key %q does not carry the account segment", result.ObjectKey)
	}
}

// 分片尺寸低于 S3 的 5MiB 下限时必须被抬回默认值：
// 迁就客户端的建议值只会让 CompleteMultipartUpload 整单 EntityTooSmall。
func TestInitMultipartRejectsUndersizedParts(t *testing.T) {
	svc, _ := newMediaService(t)

	result, err := svc.InitMultipart(context.Background(), 20<<20, 1<<20, 1<<20)
	if err != nil {
		t.Fatalf("init multipart: %v", err)
	}
	if result.PartBytes < UploadPartMinBytes {
		t.Fatalf("part bytes = %d, must not fall below the S3 floor %d", result.PartBytes, UploadPartMinBytes)
	}
}

func TestCompleteMultipartCreatesTask(t *testing.T) {
	svc, store := newMediaService(t)

	result, err := svc.InitMultipart(context.Background(), 3, 6<<20, UploadPartBytes)
	if err != nil {
		t.Fatalf("init multipart: %v", err)
	}
	// 模拟浏览器直传：字节不经过后端。
	store.PutPart(result.UploadID, 1, make([]byte, 6<<20))

	task, err := svc.CompleteMultipart(context.Background(), 3, result.ObjectKey, result.UploadID,
		[]storage.Part{{PartNumber: 1, ETag: "\"abc\""}})
	if err != nil {
		t.Fatalf("complete multipart: %v", err)
	}
	if task.Status != StatusProcessing {
		t.Fatalf("status = %q, want %q", task.Status, StatusProcessing)
	}
	if task.SourceKey != result.ObjectKey {
		t.Fatalf("source key = %q, want %q", task.SourceKey, result.ObjectKey)
	}
}

// 拼装时的对象键来自客户端，必须挡住「拿别人的 key 去拼装」。
func TestCompleteMultipartRejectsForeignKey(t *testing.T) {
	svc, store := newMediaService(t)

	result, err := svc.InitMultipart(context.Background(), 3, 6<<20, UploadPartBytes)
	if err != nil {
		t.Fatalf("init multipart: %v", err)
	}
	store.PutPart(result.UploadID, 1, make([]byte, 6<<20))

	// 账号 9 拿着账号 3 的对象键。
	_, err = svc.CompleteMultipart(context.Background(), 9, result.ObjectKey, result.UploadID,
		[]storage.Part{{PartNumber: 1, ETag: "abc"}})
	if !errors.Is(err, ErrUploadKeyRejected) {
		t.Fatalf("err = %v, want ErrUploadKeyRejected", err)
	}
}

func TestOwnsSourceKeyRejectsEscapes(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"sources/5/abc.source", true},
		{"sources/5/../6/abc.source", false},
		{"sources/6/abc.source", false},
		{"videos/5.mp4", false},
		{"sources/5/nested/abc.source", false},
		{"sources//abc.source", false},
	}
	for _, tc := range cases {
		if got := ownsSourceKey(5, tc.key); got != tc.want {
			t.Errorf("ownsSourceKey(5, %q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}
