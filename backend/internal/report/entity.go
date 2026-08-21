package report

import "time"

// TargetType 是被举报对象的类型。
// 当前只开放视频；评论与用户资料预留，届时不需要改表结构。
type TargetType string

const TargetVideo TargetType = "video"

func (t TargetType) Valid() bool { return t == TargetVideo }

// Reason 是结构化的举报理由。
//
// 用枚举而不是自由文本，有两个原因：一是同一内容的多条举报只有归一化之后
// 才能聚合成「7 人以色情举报」这种对审核员真正有用的信息；二是自由文本
// 无法在提交环节做任何校验，最终会变成没人看的垃圾字段。
type Reason string

const (
	ReasonSpam           Reason = "spam"           // 垃圾营销
	ReasonPorn           Reason = "porn"           // 色情低俗
	ReasonViolence       Reason = "violence"       // 暴力血腥
	ReasonIllegal        Reason = "illegal"        // 违法违规
	ReasonInfringement   Reason = "infringement"   // 侵权抄袭
	ReasonMisinformation Reason = "misinformation" // 虚假信息
	ReasonAbuse          Reason = "abuse"          // 人身攻击
	ReasonMinor          Reason = "minor"          // 危害未成年人
	ReasonOther          Reason = "other"          // 其他
)

func (r Reason) Valid() bool {
	switch r {
	case ReasonSpam, ReasonPorn, ReasonViolence, ReasonIllegal,
		ReasonInfringement, ReasonMisinformation, ReasonAbuse, ReasonMinor, ReasonOther:
		return true
	}
	return false
}

// Status 是**举报单自身**的处理状态。
//
// 注意与 audit.Status 区分：那个描述的是「内容能不能公开」，
// 这个描述的是「这条举报处理完了没有」。两者互不蕴含——
// 举报被 accepted 不代表内容一定下架（审核员可以只是认可举报属实），
// 内容被下架也不代表所有举报单都自动结案。混淆这两者会写出很难查的 bug。
type Status string

const (
	StatusPending   Status = "pending"   // 待人工处理
	StatusAccepted  Status = "accepted"  // 举报成立
	StatusDismissed Status = "dismissed" // 举报不成立
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusAccepted, StatusDismissed:
		return true
	}
	return false
}

// DetailMaxLength 限制补充说明长度，与列宽保持一致。
const DetailMaxLength = 500

// Report 是一条举报记录。
//
// 举报只是「通知」，落库本身**不改变任何内容的可见性**。
// 这是刻意的取舍：自动隐藏能更快压住违规内容，但也让恶意集中举报
// 可以直接封掉正常创作者，而误封的代价远高于多曝光几分钟。
// 处置权完整地留在人工手里，见 Service.Handle。
type Report struct {
	ID uint64 `gorm:"primaryKey" json:"id"`

	// 同一举报人对同一对象只能有一条记录。依赖唯一索引而不是先查后插：
	// 并发重复提交下，只有数据库约束能真正保证唯一。
	TargetType TargetType `gorm:"size:20;not null;uniqueIndex:uk_content_reports_target_reporter,priority:1;index:idx_content_reports_target,priority:1" json:"target_type"`
	TargetID   uint64     `gorm:"not null;uniqueIndex:uk_content_reports_target_reporter,priority:2;index:idx_content_reports_target,priority:2" json:"target_id"`
	ReporterID uint64     `gorm:"not null;uniqueIndex:uk_content_reports_target_reporter,priority:3;index:idx_content_reports_reporter_created,priority:1" json:"reporter_id"`

	Reason Reason `gorm:"size:20;not null" json:"reason"`
	Detail string `gorm:"size:500" json:"detail"`

	// 待处理队列按 (status, id) 扫描，因此状态放进联合索引首位。
	Status Status `gorm:"size:20;not null;default:'pending';index:idx_content_reports_status_id,priority:1" json:"status"`

	// HandlerID / HandledAt / HandleNote 仅在处置后有值。
	HandlerID  uint64     `gorm:"not null;default:0" json:"handler_id"`
	HandledAt  *time.Time `json:"handled_at"`
	HandleNote string     `gorm:"size:500" json:"handle_note"`

	CreatedAt time.Time `gorm:"index:idx_content_reports_reporter_created,priority:2" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Report) TableName() string { return "content_reports" }

// Action 是审核员对一个被举报对象的处置动作。
type Action string

const (
	// ActionDismiss 驳回：举报不成立，内容保持原状。
	ActionDismiss Action = "dismiss"
	// ActionTakedown 下架：内容确实违规，转为审核拒绝状态并退出公开信息流。
	ActionTakedown Action = "takedown"
)

func (a Action) Valid() bool { return a == ActionDismiss || a == ActionTakedown }

// SubmitRequest 是用户提交举报的入参。
type SubmitRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
	Reason  Reason `json:"reason" binding:"required"`
	Detail  string `json:"detail"`
}

// ListMineRequest 分页查询自己提交过的举报。
type ListMineRequest struct {
	Limit    int    `json:"limit"`
	OffsetID uint64 `json:"offset_id"`
}

// ListPendingRequest 是审核员拉取待处理队列的入参。
type ListPendingRequest struct {
	Limit int `json:"limit"`
}

// HandleRequest 是审核员的处置入参。
//
// 以 (target_type, target_id) 而非单条举报 ID 为单位：同一内容往往有多条举报，
// 审核员做的是「对这个内容下结论」，逐条处置既低效又会产生自相矛盾的结果。
type HandleRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
	Action  Action `json:"action" binding:"required"`
	Note    string `json:"note"`
}

// PendingItem 是待处理队列里的一项，按被举报对象聚合。
type PendingItem struct {
	TargetType   TargetType       `json:"target_type"`
	TargetID     uint64           `json:"target_id"`
	ReportCount  int64            `json:"report_count"`
	ReasonCounts map[Reason]int64 `json:"reason_counts"`
	FirstlyAt    time.Time        `json:"firstly_at"`
	LatestAt     time.Time        `json:"latest_at"`
	// Samples 是若干条补充说明原文，供审核员判断，最多取几条避免响应膨胀。
	Samples []string `json:"samples"`
}
