package report

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeContent 记录被调用情况，用来断言「提交举报不触碰内容」这条硬约束。
type fakeContent struct {
	authors        map[uint64]uint64
	takedownCalls  []uint64
	takedownErr    error
	loadForReportE error
}

func (f *fakeContent) LoadForReport(_ uint64, targetID uint64) (uint64, bool, error) {
	if f.loadForReportE != nil {
		return 0, false, f.loadForReportE
	}
	author, ok := f.authors[targetID]
	return author, ok, nil
}

func (f *fakeContent) Takedown(_ context.Context, targetID uint64, _ uint64, _ string) error {
	if f.takedownErr != nil {
		return f.takedownErr
	}
	f.takedownCalls = append(f.takedownCalls, targetID)
	return nil
}

func newTestService(t *testing.T, content *fakeContent, reviewers ...uint64) (*Service, *gorm.DB) {
	t.Helper()

	// 每个用例独立的内存库，避免共享 cache 造成用例间串味。
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Report{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewService(db, content, reviewers), db
}

func defaultContent() *fakeContent {
	return &fakeContent{authors: map[uint64]uint64{10: 100, 11: 101}}
}

func TestSubmitRecordsReportWithoutTouchingContent(t *testing.T) {
	content := defaultContent()
	service, _ := newTestService(t, content)

	row, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonPorn})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if row.Status != StatusPending {
		t.Fatalf("status = %q, want %q", row.Status, StatusPending)
	}

	// 这是整个功能最重要的一条约束：举报只是通知，绝不自动处置。
	if len(content.takedownCalls) != 0 {
		t.Fatalf("submit triggered %d takedown call(s), reports must never auto-hide content", len(content.takedownCalls))
	}
}

func TestSubmitRejectsDuplicateFromSameReporter(t *testing.T) {
	service, _ := newTestService(t, defaultContent())

	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonAbuse}); !errors.Is(err, ErrAlreadyReported) {
		t.Fatalf("second submit = %v, want ErrAlreadyReported", err)
	}

	// 换个人举报同一内容应当照常受理。
	if _, err := service.Submit(8, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("other reporter submit: %v", err)
	}
	// 同一人举报另一个内容也应当照常受理。
	if _, err := service.Submit(7, SubmitRequest{VideoID: 11, Reason: ReasonSpam}); err != nil {
		t.Fatalf("other target submit: %v", err)
	}
}

func TestSubmitValidation(t *testing.T) {
	cases := []struct {
		name       string
		reporterID uint64
		req        SubmitRequest
		want       error
	}{
		{"举报自己", 100, SubmitRequest{VideoID: 10, Reason: ReasonSpam}, ErrSelfReport},
		{"内容不存在", 7, SubmitRequest{VideoID: 999, Reason: ReasonSpam}, ErrContentNotFound},
		{"理由非法", 7, SubmitRequest{VideoID: 10, Reason: Reason("whatever")}, ErrInvalidReason},
		{"其他但未说明", 7, SubmitRequest{VideoID: 10, Reason: ReasonOther}, ErrDetailRequired},
		{"其他且只有空白", 7, SubmitRequest{VideoID: 10, Reason: ReasonOther, Detail: "   "}, ErrDetailRequired},
		{"说明超长", 7, SubmitRequest{VideoID: 10, Reason: ReasonSpam, Detail: string(make([]rune, DetailMaxLength+1))}, ErrDetailTooLong},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newTestService(t, defaultContent())
			if _, err := service.Submit(tc.reporterID, tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestHandleTakedownResolvesAllPendingReports(t *testing.T) {
	content := defaultContent()
	service, db := newTestService(t, content, 1)

	for _, reporter := range []uint64{7, 8, 9} {
		if _, err := service.Submit(reporter, SubmitRequest{VideoID: 10, Reason: ReasonPorn}); err != nil {
			t.Fatalf("submit by %d: %v", reporter, err)
		}
	}

	result, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionTakedown, Note: "确认违规"})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.ReportCount != 3 {
		t.Fatalf("resolved = %d, want 3", result.ReportCount)
	}
	if len(content.takedownCalls) != 1 || content.takedownCalls[0] != 10 {
		t.Fatalf("takedown calls = %v, want [10]", content.takedownCalls)
	}

	var rows []Report
	if err := db.Where("target_id = ?", 10).Find(&rows).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, row := range rows {
		if row.Status != StatusAccepted {
			t.Fatalf("report %d status = %q, want %q", row.ID, row.Status, StatusAccepted)
		}
		if row.HandlerID != 1 || row.HandledAt == nil {
			t.Fatalf("report %d missing handler trail: handler=%d handledAt=%v", row.ID, row.HandlerID, row.HandledAt)
		}
	}
}

func TestHandleDismissLeavesContentAlone(t *testing.T) {
	content := defaultContent()
	service, db := newTestService(t, content, 1)

	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionDismiss}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(content.takedownCalls) != 0 {
		t.Fatalf("dismiss triggered takedown: %v", content.takedownCalls)
	}

	var row Report
	if err := db.Where("target_id = ?", 10).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.Status != StatusDismissed {
		t.Fatalf("status = %q, want %q", row.Status, StatusDismissed)
	}
}

// 下架失败时举报必须留在队列里，否则内容还在线上、举报却已被标记处理完毕。
func TestHandleKeepsReportsPendingWhenTakedownFails(t *testing.T) {
	content := defaultContent()
	content.takedownErr = errors.New("boom")
	service, db := newTestService(t, content, 1)

	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonPorn}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionTakedown}); err == nil {
		t.Fatal("handle succeeded despite takedown failure")
	}

	var row Report
	if err := db.Where("target_id = ?", 10).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.Status != StatusPending {
		t.Fatalf("status = %q, want %q so the report stays visible to reviewers", row.Status, StatusPending)
	}
}

func TestHandleRejectsNonReviewerAndEmptyQueue(t *testing.T) {
	service, _ := newTestService(t, defaultContent(), 1)

	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := service.Handle(context.Background(), 42, HandleRequest{VideoID: 10, Action: ActionDismiss}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("non-reviewer handle = %v, want ErrNotReviewer", err)
	}
	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 11, Action: ActionDismiss}); !errors.Is(err, ErrNothingPending) {
		t.Fatalf("empty queue handle = %v, want ErrNothingPending", err)
	}
	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: Action("ban")}); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("invalid action = %v, want ErrInvalidAction", err)
	}

	// 处置过一次之后队列应当为空，重复处置不能再次生效。
	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionDismiss}); err != nil {
		t.Fatalf("first dismiss: %v", err)
	}
	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionDismiss}); !errors.Is(err, ErrNothingPending) {
		t.Fatalf("repeat dismiss = %v, want ErrNothingPending", err)
	}
}

func TestListPendingAggregatesByTarget(t *testing.T) {
	content := &fakeContent{authors: map[uint64]uint64{10: 100, 11: 101}}
	service, _ := newTestService(t, content, 1)

	// 视频 10 被 3 人举报，视频 11 被 1 人举报。
	for _, reporter := range []uint64{7, 8, 9} {
		reason := ReasonPorn
		if reporter == 9 {
			reason = ReasonSpam
		}
		if _, err := service.Submit(reporter, SubmitRequest{VideoID: 10, Reason: reason, Detail: "说明"}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if _, err := service.Submit(7, SubmitRequest{VideoID: 11, Reason: ReasonAbuse}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	items, err := service.ListPending(1, ListPendingRequest{})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	// 举报数多的排前面，审核员先看最需要处理的。
	if items[0].TargetID != 10 || items[0].ReportCount != 3 {
		t.Fatalf("first item = target %d count %d, want target 10 count 3", items[0].TargetID, items[0].ReportCount)
	}
	if items[0].ReasonCounts[ReasonPorn] != 2 || items[0].ReasonCounts[ReasonSpam] != 1 {
		t.Fatalf("reason counts = %v, want porn:2 spam:1", items[0].ReasonCounts)
	}
	if len(items[0].Samples) != maxSamplesPerTarget {
		t.Fatalf("samples = %d, want %d", len(items[0].Samples), maxSamplesPerTarget)
	}

	if _, err := service.ListPending(42, ListPendingRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("non-reviewer list = %v, want ErrNotReviewer", err)
	}
}

func TestListMineReturnsOwnReportsWithOutcome(t *testing.T) {
	service, _ := newTestService(t, defaultContent(), 1)

	if _, err := service.Submit(7, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := service.Submit(8, SubmitRequest{VideoID: 10, Reason: ReasonSpam}); err != nil {
		t.Fatalf("submit by other: %v", err)
	}

	mine, err := service.ListMine(7, ListMineRequest{})
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(mine) != 1 || mine[0].ReporterID != 7 {
		t.Fatalf("list mine returned %d rows, want only reporter 7's own report", len(mine))
	}

	if _, err := service.Handle(context.Background(), 1, HandleRequest{VideoID: 10, Action: ActionDismiss}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	// 举报人必须能看到结论，否则举报就成了没有回音的许愿池。
	after, err := service.ListMine(7, ListMineRequest{})
	if err != nil {
		t.Fatalf("list mine after handling: %v", err)
	}
	if after[0].Status != StatusDismissed || after[0].HandledAt == nil {
		t.Fatalf("status = %q handledAt = %v, want dismissed with timestamp", after[0].Status, after[0].HandledAt)
	}
}
