package danmaku

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/video"
)

type fakeAccess struct {
	visible map[uint64]bool
}

func (f fakeAccess) EnsureVisible(_ uint64, videoID uint64) error {
	if f.visible[videoID] {
		return nil
	}
	return video.ErrVideoNotFound
}

func setupService(t *testing.T, visible ...uint64) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:danmaku-" + t.Name() + "?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&VideoDanmaku{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	allowed := make(map[uint64]bool, len(visible))
	for _, id := range visible {
		allowed[id] = true
	}
	return NewService(db, fakeAccess{visible: allowed})
}

func TestSendAndList(t *testing.T) {
	svc := setupService(t, 7)

	first, err := svc.Send(2, "alice", SendRequest{VideoID: 7, Content: "好看", OffsetMs: 1200})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if first.ID == 0 || first.Content != "好看" || first.OffsetMs != 1200 {
		t.Fatalf("unexpected first item: %+v", first)
	}

	_, err = svc.Send(3, "bob", SendRequest{VideoID: 7, Content: "  前排  ", OffsetMs: 400})
	if err != nil {
		t.Fatalf("send second: %v", err)
	}

	items, err := svc.List(0, ListRequest{VideoID: 7})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].Content != "前排" || items[0].OffsetMs != 400 {
		t.Fatalf("expected earlier offset first, got %+v", items[0])
	}
	if items[1].ID != first.ID {
		t.Fatalf("expected later offset second, got %+v", items[1])
	}
}

func TestSendRejectsInvalidInput(t *testing.T) {
	svc := setupService(t, 7)

	cases := []struct {
		name string
		req  SendRequest
		want error
	}{
		{name: "empty", req: SendRequest{VideoID: 7, Content: "   ", OffsetMs: 0}, want: ErrEmptyContent},
		{name: "too long", req: SendRequest{VideoID: 7, Content: strings.Repeat("啊", MaxContentRunes+1), OffsetMs: 0}, want: ErrContentTooLong},
		{name: "negative offset", req: SendRequest{VideoID: 7, Content: "hi", OffsetMs: -1}, want: ErrInvalidOffset},
		{name: "huge offset", req: SendRequest{VideoID: 7, Content: "hi", OffsetMs: MaxOffsetMs + 1}, want: ErrInvalidOffset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Send(2, "alice", tc.req)
			if err != tc.want {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestInvisibleVideoLooksMissing(t *testing.T) {
	svc := setupService(t, 7)

	_, err := svc.List(9, ListRequest{VideoID: 99})
	if err != video.ErrVideoNotFound {
		t.Fatalf("list missing: %v", err)
	}
	_, err = svc.Send(9, "alice", SendRequest{VideoID: 99, Content: "探路", OffsetMs: 0})
	if err != video.ErrVideoNotFound {
		t.Fatalf("send missing: %v", err)
	}
}
