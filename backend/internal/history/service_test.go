package history

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/audit"
	"my_feed_system/internal/video"
)

type fakeVideos struct {
	videos map[uint64]*video.Video
}

func (f fakeVideos) GetDetail(viewerID uint64, req video.GetDetailRequest) (*video.Video, error) {
	item := f.videos[req.ID]
	if item == nil {
		return nil, video.ErrVideoNotFound
	}
	if item.AuditStatus.IsPublic() || (viewerID != 0 && viewerID == item.AuthorID) {
		return item, nil
	}
	return nil, video.ErrVideoNotFound
}

func setupService(t *testing.T) (*Service, fakeVideos) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:history-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&WatchHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	videos := fakeVideos{videos: map[uint64]*video.Video{
		7:  {ID: 7, AuthorID: 2, Title: "公开", AuditStatus: audit.StatusApproved},
		8:  {ID: 8, AuthorID: 2, Title: "待审", AuditStatus: audit.StatusPending},
		9:  {ID: 9, AuthorID: 3, Title: "别人待审", AuditStatus: audit.StatusPending},
		10: {ID: 10, AuthorID: 4, Title: "另一条公开", AuditStatus: audit.StatusApproved},
	}}
	return NewService(db, videos), videos
}

func TestUpsertSkipsShallowAndSavesMidway(t *testing.T) {
	svc, _ := setupService(t)

	skipped, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: 1000, DurationMs: 20000})
	if err != nil {
		t.Fatalf("shallow upsert: %v", err)
	}
	if skipped.Saved {
		t.Fatal("shallow watch must not persist")
	}

	saved, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: 8000, DurationMs: 20000})
	if err != nil {
		t.Fatalf("midway upsert: %v", err)
	}
	if !saved.Saved || saved.Item == nil || saved.Item.Completed || saved.Item.ResumeMs != 8000 {
		t.Fatalf("unexpected saved item: %+v", saved.Item)
	}
}

func TestUpsertCompletedClearsPosition(t *testing.T) {
	svc, _ := setupService(t)

	saved, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: 14800, DurationMs: 15000})
	if err != nil {
		t.Fatalf("complete upsert: %v", err)
	}
	if !saved.Item.Completed || saved.Item.PositionMs != 0 || saved.Item.ResumeMs != 0 {
		t.Fatalf("completed should reset position: %+v", saved.Item)
	}

	// 循环回零后的 0 进度不得把已看完改回未看完。
	again, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: 0, DurationMs: 15000})
	if err != nil {
		t.Fatalf("zero upsert: %v", err)
	}
	if again.Saved {
		t.Fatal("zero progress must not overwrite")
	}

	listed, err := svc.List(11, ListRequest{Status: StatusCompleted})
	if err != nil {
		t.Fatalf("list completed: %v", err)
	}
	if len(listed.Items) != 1 || !listed.Items[0].Completed {
		t.Fatalf("expected one completed item, got %+v", listed.Items)
	}
}

func TestUpsertHiddenVideoLooksMissing(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.Upsert(11, UpsertRequest{VideoID: 9, PositionMs: 8000, DurationMs: 20000})
	if !errors.Is(err, video.ErrVideoNotFound) {
		t.Fatalf("hidden upsert: %v", err)
	}
}

func TestListOmitsInvisibleAndPages(t *testing.T) {
	svc, _ := setupService(t)

	if _, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: 8000, DurationMs: 20000}); err != nil {
		t.Fatalf("upsert 7: %v", err)
	}
	if _, err := svc.Upsert(11, UpsertRequest{VideoID: 10, PositionMs: 9000, DurationMs: 20000}); err != nil {
		t.Fatalf("upsert 10: %v", err)
	}

	first, err := svc.List(11, ListRequest{Status: StatusUnfinished, Limit: 1})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if !first.HasMore || first.NextCursor == "" || len(first.Items) != 1 {
		t.Fatalf("expected a page of 1 with more, got %+v", first)
	}

	second, err := svc.List(11, ListRequest{Status: StatusUnfinished, Limit: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if second.HasMore || len(second.Items) != 1 {
		t.Fatalf("expected last page, got %+v", second)
	}
	if first.Items[0].VideoID == second.Items[0].VideoID {
		t.Fatal("pages should not repeat the same video")
	}

	_, err = svc.List(11, ListRequest{Status: "watching"})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("bad status: %v", err)
	}
}

func TestProgressOmitsHidden(t *testing.T) {
	svc, _ := setupService(t)
	if _, err := svc.Upsert(2, UpsertRequest{VideoID: 8, PositionMs: 4000, DurationMs: 10000}); err != nil {
		t.Fatalf("author upsert pending: %v", err)
	}

	mine, err := svc.Progress(2, ProgressRequest{VideoIDs: []uint64{8, 9}})
	if err != nil {
		t.Fatalf("author progress: %v", err)
	}
	if len(mine) != 1 || mine[0].VideoID != 8 {
		t.Fatalf("author should only see own pending: %+v", mine)
	}

	other, err := svc.Progress(11, ProgressRequest{VideoIDs: []uint64{8, 9}})
	if err != nil {
		t.Fatalf("other progress: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("stranger must not learn hidden videos: %+v", other)
	}
}

func TestParseCursor(t *testing.T) {
	if _, _, err := parseCursor("abc"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("bad cursor: %v", err)
	}
	encoded := encodeCursor(1_700_000_000_000, 9)
	gotMs, gotID, err := parseCursor(encoded)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if gotMs != 1_700_000_000_000 || gotID != 9 {
		t.Fatalf("cursor decoded to %d/%d", gotMs, gotID)
	}
}

func TestTooManyProgressIDs(t *testing.T) {
	svc, _ := setupService(t)
	ids := make([]uint64, MaxProgressIDs+1)
	for i := range ids {
		ids[i] = uint64(i + 1)
	}
	_, err := svc.Progress(11, ProgressRequest{VideoIDs: ids})
	if !errors.Is(err, ErrTooManyIDs) {
		t.Fatalf("too many: %v", err)
	}
}

func TestInvalidProgressRejected(t *testing.T) {
	svc, _ := setupService(t)
	_, err := svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: -1, DurationMs: 1000})
	if !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("negative: %v", err)
	}
	_, err = svc.Upsert(11, UpsertRequest{VideoID: 7, PositionMs: MaxOffsetMs + 1, DurationMs: 1000})
	if !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("huge: %v", err)
	}
}
