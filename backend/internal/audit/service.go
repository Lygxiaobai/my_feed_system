package audit

import (
	"context"
	"errors"
	"log/slog"

	"gorm.io/gorm"
)

var (
	// ErrTargetNotFound 表示待审内容不存在，通常是审核事件到达时内容已被删除。
	ErrTargetNotFound = errors.New("audit target not found")
	// ErrNotReviewable 表示该内容当前状态不接受人工处置。
	ErrNotReviewable = errors.New("audit target is not awaiting review")
	// ErrNotReviewer 表示操作者没有审核权限。
	ErrNotReviewer = errors.New("account is not an auditor")
)

// StatusStore 由内容所属模块实现，用于读写业务表上的审核状态。
//
// 这样 audit 包只依赖一个窄接口，不需要 import video 包——
// 而 video 包因为要用 Status 类型必须 import audit，反向依赖会形成环。
type StatusStore interface {
	// LoadForAudit 读取待审内容的字段快照。内容不存在时返回 nil。
	LoadForAudit(targetID uint64) (*Target, error)
	// UpdateStatus 在事务内更新审核状态。
	UpdateStatus(tx *gorm.DB, targetID uint64, status Status) error
	// ListByStatus 按状态分页列出内容，供人工复审队列使用。
	ListByStatus(status Status, limit int, offsetID uint64) ([]Target, error)
}

// Target 是待审内容的字段快照，与具体业务实体解耦。
type Target struct {
	ID          uint64
	AuthorID    uint64
	Username    string
	Title       string
	Description string
	PlayURL     string
	CoverURL    string
	Status      Status
}

// ApprovalPublisher 在内容通过审核后投递公开化事件。
//
// 由外部实现并注入，audit 包因此不需要认识 outbox 与 mq 的具体类型。
// 必须接受事务参数：事件要和状态变更一起提交，否则会出现
// 「状态已通过但事件丢失」或「事件已发但状态回滚」。
type ApprovalPublisher interface {
	EnqueueApproved(tx *gorm.DB, videoID uint64, authorID uint64) error
}

// Service 编排审核判定、状态变更与流水记录。
type Service struct {
	db        *gorm.DB
	repo      *Repo
	store     StatusStore
	moderator Moderator
	publisher ApprovalPublisher
	// reviewers 是有人工审核权限的账号。用配置化的白名单而不是引入
	// 半成品的角色系统：当前只有「是/不是审核员」这一个区分，
	// 为它铺一套 RBAC 属于过度设计。
	reviewers map[uint64]struct{}
}

func NewService(db *gorm.DB, store StatusStore, moderator Moderator, publisher ApprovalPublisher, reviewerIDs []uint64) *Service {
	reviewers := make(map[uint64]struct{}, len(reviewerIDs))
	for _, id := range reviewerIDs {
		reviewers[id] = struct{}{}
	}
	return &Service{
		db:        db,
		repo:      NewRepo(db),
		store:     store,
		moderator: moderator,
		publisher: publisher,
		reviewers: reviewers,
	}
}

// IsReviewer 判断账号是否具备人工审核权限。
func (s *Service) IsReviewer(accountID uint64) bool {
	_, ok := s.reviewers[accountID]
	return ok
}

// ModerateVideo 对一个视频执行机审，落库最终状态与流水。
//
// 返回最终状态供调用方决定后续动作：只有 StatusApproved 才应该触发
// 推送时间线、计入热度这些让内容公开可见的副作用。
func (s *Service) ModerateVideo(ctx context.Context, videoID uint64) (Status, error) {
	target, err := s.store.LoadForAudit(videoID)
	if err != nil {
		return "", err
	}
	if target == nil {
		return "", ErrTargetNotFound
	}
	// 人工已经处置过的内容不再被机审覆盖，避免重复投递的事件把结论改回去。
	if target.Status == StatusApproved || target.Status == StatusRejected {
		return target.Status, nil
	}

	result := s.evaluate(ctx, target)
	next := statusFor(result.Decision)

	if err := s.apply(target, next, SourceMachine, s.moderator.Name(), 0, result); err != nil {
		return "", err
	}

	slog.InfoContext(ctx, "video moderated",
		slog.Uint64("video_id", videoID),
		slog.String("decision", string(result.Decision)),
		slog.String("status", string(next)),
		slog.String("label", result.Label))

	return next, nil
}

// evaluate 依次执行文本与媒体审核，任一环节拒绝即整体拒绝。
//
// 审核链路自身出错时返回 Review 而不是 Pass：
// 这与限流中间件的降级取向相反——限流故障应放行以免拖垮业务，
// 审核故障若放行则直接构成合规事故，必须 fail-closed。
func (s *Service) evaluate(ctx context.Context, target *Target) Result {
	text := target.Title
	if target.Description != "" {
		text += "\n" + target.Description
	}

	textResult, err := s.moderator.ModerateText(ctx, text)
	if err != nil {
		slog.ErrorContext(ctx, "text moderation failed, falling back to manual review",
			slog.Uint64("video_id", target.ID), slog.String("error", err.Error()))
		return Review("error", "文本审核调用失败: "+err.Error())
	}
	if textResult.Decision != DecisionPass {
		return textResult
	}

	mediaResult, err := s.moderator.ModerateMedia(ctx, target.PlayURL, target.CoverURL)
	if err != nil {
		slog.ErrorContext(ctx, "media moderation failed, falling back to manual review",
			slog.Uint64("video_id", target.ID), slog.String("error", err.Error()))
		return Review("error", "媒体审核调用失败: "+err.Error())
	}
	return mediaResult
}

// ManualReview 处理人工复审结论。
func (s *Service) ManualReview(ctx context.Context, operatorID uint64, videoID uint64, approve bool, note string) (Status, error) {
	if !s.IsReviewer(operatorID) {
		return "", ErrNotReviewer
	}

	target, err := s.store.LoadForAudit(videoID)
	if err != nil {
		return "", err
	}
	if target == nil {
		return "", ErrTargetNotFound
	}
	// 只允许处置尚未有终态结论的内容，避免并发下两个审核员互相覆盖。
	if target.Status != StatusReviewing && target.Status != StatusPending {
		return "", ErrNotReviewable
	}

	next := StatusRejected
	if approve {
		next = StatusApproved
	}

	result := Result{Decision: DecisionBlock, Label: "manual", Detail: note}
	if approve {
		result.Decision = DecisionPass
	}

	if err := s.apply(target, next, SourceManual, "manual", operatorID, result); err != nil {
		return "", err
	}

	slog.InfoContext(ctx, "manual review recorded",
		slog.Uint64("video_id", videoID),
		slog.Uint64("operator_id", operatorID),
		slog.String("status", string(next)))

	return next, nil
}

// ListReviewing 返回需要人工处置的内容。
//
// 同时包含 reviewing 与 pending 两种状态，而不只是前者：
// pending 意味着机审尚未给出结论，正常情况下只停留数秒；但如果审核事件
// 因 worker 故障或消息丢失而没送达，内容会永久停在 pending——
// 既不公开、也不在队列里，等于被系统遗忘。把它们一并列出，
// 保证任何非终态内容最终都会落到人的视野里。
func (s *Service) ListReviewing(limit int, offsetID uint64) ([]Target, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	targets, err := s.store.ListByStatus(StatusReviewing, limit, offsetID)
	if err != nil {
		return nil, err
	}
	if len(targets) >= limit {
		return targets, nil
	}

	pending, err := s.store.ListByStatus(StatusPending, limit-len(targets), offsetID)
	if err != nil {
		return nil, err
	}
	return append(targets, pending...), nil
}

// History 返回某个视频的完整处置链路。
func (s *Service) History(videoID uint64) ([]Record, error) {
	return s.repo.ListByTarget(TargetVideo, videoID)
}

// apply 在同一事务里更新状态、写入流水，并在通过时投递公开化事件。
//
// 三件事必须同事务：状态变了但没流水就无法追溯；通过了但没投递事件，
// 内容就会停在「已通过却不出现在信息流」的状态——机审与人工两条路径
// 都走这里，就不会出现只修好一条的情况。
func (s *Service) apply(target *Target, next Status, source Source, moderator string, operatorID uint64, result Result) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.store.UpdateStatus(tx, target.ID, next); err != nil {
			return err
		}
		if err := s.repo.Append(tx, &Record{
			TargetType: TargetVideo,
			TargetID:   target.ID,
			FromStatus: target.Status,
			ToStatus:   next,
			Source:     source,
			Moderator:  moderator,
			OperatorID: operatorID,
			Label:      result.Label,
			Detail:     truncate(result.Detail, 1000),
		}); err != nil {
			return err
		}

		if next != StatusApproved || s.publisher == nil {
			return nil
		}
		return s.publisher.EnqueueApproved(tx, target.ID, target.AuthorID)
	})
}

func statusFor(decision Decision) Status {
	switch decision {
	case DecisionPass:
		return StatusApproved
	case DecisionBlock:
		return StatusRejected
	default:
		return StatusReviewing
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
