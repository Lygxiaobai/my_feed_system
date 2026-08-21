package report

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	// ErrContentNotFound 表示被举报内容不存在，或对举报人不可见。
	ErrContentNotFound = errors.New("report target not found")
	// ErrSelfReport 表示试图举报自己的内容。
	ErrSelfReport = errors.New("cannot report own content")
	// ErrInvalidReason 表示举报理由不在枚举内。
	ErrInvalidReason = errors.New("invalid report reason")
	// ErrDetailRequired 表示选择「其他」时未填写补充说明。
	ErrDetailRequired = errors.New("report detail is required for this reason")
	// ErrDetailTooLong 表示补充说明超长。
	ErrDetailTooLong = errors.New("report detail is too long")
	// ErrAlreadyReported 表示同一人已举报过同一对象。
	ErrAlreadyReported = errors.New("content already reported by this account")
	// ErrNotReviewer 表示操作者没有处置权限。
	ErrNotReviewer = errors.New("account is not a reviewer")
	// ErrInvalidAction 表示处置动作不合法。
	ErrInvalidAction = errors.New("invalid handle action")
	// ErrNothingPending 表示该对象没有待处理的举报。
	ErrNothingPending = errors.New("no pending report for target")
)

// 待处理队列里每个对象最多附带几条补充说明，避免热点内容把响应撑爆。
const maxSamplesPerTarget = 3

// ContentStore 由内容所属模块实现，report 包因此不需要认识 video 的内部结构。
//
// 反向依赖是不行的：video 已经 import audit，而处置要写 audit 流水，
// 让 report 定义窄接口、由 video 提供实现，是这套代码里既有的做法
// （参见 audit.StatusStore 与 audit.ApprovalPublisher）。
type ContentStore interface {
	// LoadForReport 返回内容作者 ID。内容不存在、或对该查看者不可见时返回 ok=false。
	//
	// 带上 viewerID 是有意的：只能举报自己看得见的内容。
	// 否则举报接口会变成一个「探测某 ID 的内容是否存在」的旁路。
	LoadForReport(viewerID uint64, targetID uint64) (authorID uint64, ok bool, err error)

	// Takedown 下架内容：转为审核拒绝状态、留存处置流水、失效详情缓存。
	Takedown(ctx context.Context, targetID uint64, operatorID uint64, note string) error
}

// Service 编排举报的提交、查询与人工处置。
type Service struct {
	db      *gorm.DB
	repo    *Repo
	content ContentStore
	// reviewers 复用审核配置里的白名单，不另立一套权限体系。
	reviewers map[uint64]struct{}
}

func NewService(db *gorm.DB, content ContentStore, reviewerIDs []uint64) *Service {
	reviewers := make(map[uint64]struct{}, len(reviewerIDs))
	for _, id := range reviewerIDs {
		reviewers[id] = struct{}{}
	}
	return &Service{
		db:        db,
		repo:      NewRepo(db),
		content:   content,
		reviewers: reviewers,
	}
}

// IsReviewer 判断账号是否具备处置权限。
func (s *Service) IsReviewer(accountID uint64) bool {
	_, ok := s.reviewers[accountID]
	return ok
}

// Submit 记录一条举报。
//
// 落库之后**不做任何影响内容可见性的事**，也不触发自动处置。
// 想加「N 次举报自动隐藏」的话请先想清楚：那等价于把封禁权交给了
// 任何能凑出 N 个账号的人。
func (s *Service) Submit(reporterID uint64, req SubmitRequest) (*Report, error) {
	if !req.Reason.Valid() {
		return nil, ErrInvalidReason
	}

	detail := strings.TrimSpace(req.Detail)
	if utf8.RuneCountInString(detail) > DetailMaxLength {
		return nil, ErrDetailTooLong
	}
	// 「其他」没有预置语义，不写明理由的话审核员无从判断，等于一条废举报。
	if req.Reason == ReasonOther && detail == "" {
		return nil, ErrDetailRequired
	}

	authorID, ok, err := s.content.LoadForReport(reporterID, req.VideoID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrContentNotFound
	}
	if authorID == reporterID {
		return nil, ErrSelfReport
	}

	row := &Report{
		TargetType: TargetVideo,
		TargetID:   req.VideoID,
		ReporterID: reporterID,
		Reason:     req.Reason,
		Detail:     detail,
		Status:     StatusPending,
	}

	inserted, err := s.repo.Create(row)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, ErrAlreadyReported
	}

	slog.Info("content reported",
		slog.Uint64("video_id", req.VideoID),
		slog.Uint64("reporter_id", reporterID),
		slog.String("reason", string(req.Reason)))

	return row, nil
}

// ListMine 返回举报人自己提交过的举报及其结论。
//
// 这个接口不是可有可无的：举报提交后如果永远看不到下文，
// 用户无从判断平台是否处理过，举报机制就退化成了许愿池。
func (s *Service) ListMine(reporterID uint64, req ListMineRequest) ([]Report, error) {
	return s.repo.ListByReporter(reporterID, normalizeLimit(req.Limit), req.OffsetID)
}

// ListPending 返回按被举报对象聚合的待处理队列。
func (s *Service) ListPending(operatorID uint64, req ListPendingRequest) ([]PendingItem, error) {
	if !s.IsReviewer(operatorID) {
		return nil, ErrNotReviewer
	}

	// 明细条数上限放宽于对象数上限：一个对象可能有很多条举报。
	rows, err := s.repo.ListPending(normalizeLimit(req.Limit) * 20)
	if err != nil {
		return nil, err
	}
	return aggregatePending(rows, normalizeLimit(req.Limit)), nil
}

// Handle 提交对某个被举报对象的处置结论。
func (s *Service) Handle(ctx context.Context, operatorID uint64, req HandleRequest) (*PendingItem, error) {
	if !s.IsReviewer(operatorID) {
		return nil, ErrNotReviewer
	}
	if !req.Action.Valid() {
		return nil, ErrInvalidAction
	}

	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > DetailMaxLength {
		return nil, ErrDetailTooLong
	}

	pending, err := s.repo.CountPendingByTarget(TargetVideo, req.VideoID)
	if err != nil {
		return nil, err
	}
	if pending == 0 {
		return nil, ErrNothingPending
	}

	// 先下架再结案。反过来的话，若下架失败，举报单已经被标成处理完毕，
	// 内容却还在线上——那是一条没人会再看的漏网记录。
	// 当前顺序下最坏情况只是举报单继续留在队列里，审核员会再看到它。
	if req.Action == ActionTakedown {
		if err := s.content.Takedown(ctx, req.VideoID, operatorID, note); err != nil {
			return nil, err
		}
	}

	status := StatusDismissed
	if req.Action == ActionTakedown {
		status = StatusAccepted
	}

	resolved, err := s.repo.ResolveTarget(nil, TargetVideo, req.VideoID, status, operatorID, note, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "report handled",
		slog.Uint64("video_id", req.VideoID),
		slog.Uint64("operator_id", operatorID),
		slog.String("action", string(req.Action)),
		slog.Int64("resolved_reports", resolved))

	return &PendingItem{
		TargetType:  TargetVideo,
		TargetID:    req.VideoID,
		ReportCount: resolved,
	}, nil
}

// aggregatePending 把举报明细按对象归并，保持首次举报时间的先后顺序。
func aggregatePending(rows []Report, maxTargets int) []PendingItem {
	index := make(map[uint64]*PendingItem, len(rows))
	order := make([]uint64, 0, len(rows))

	for _, row := range rows {
		item, ok := index[row.TargetID]
		if !ok {
			item = &PendingItem{
				TargetType:   row.TargetType,
				TargetID:     row.TargetID,
				ReasonCounts: map[Reason]int64{},
				FirstlyAt:    row.CreatedAt,
				LatestAt:     row.CreatedAt,
			}
			index[row.TargetID] = item
			order = append(order, row.TargetID)
		}

		item.ReportCount++
		item.ReasonCounts[row.Reason]++
		if row.CreatedAt.Before(item.FirstlyAt) {
			item.FirstlyAt = row.CreatedAt
		}
		if row.CreatedAt.After(item.LatestAt) {
			item.LatestAt = row.CreatedAt
		}
		if row.Detail != "" && len(item.Samples) < maxSamplesPerTarget {
			item.Samples = append(item.Samples, row.Detail)
		}
	}

	items := make([]PendingItem, 0, len(order))
	for _, id := range order {
		items = append(items, *index[id])
	}

	// 举报数多的排前面，同数量时先来的先处理。
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ReportCount != items[j].ReportCount {
			return items[i].ReportCount > items[j].ReportCount
		}
		return items[i].FirstlyAt.Before(items[j].FirstlyAt)
	})

	if len(items) > maxTargets {
		items = items[:maxTargets]
	}
	return items
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 20
	}
	return limit
}
