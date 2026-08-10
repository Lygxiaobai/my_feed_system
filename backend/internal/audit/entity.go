package audit

import "time"

// Status 是内容的审核状态，取代「发布即可见」的旧行为。
//
// 状态流转（发布后仅作者可见，通过后才进入公开信息流）：
//
//	StatusPending ──机审通过──▶ StatusApproved  进入信息流
//	      │
//	      ├────机审拒绝──▶ StatusRejected  仅作者可见，附通用原因
//	      │
//	      └────机审存疑或失败──▶ StatusReviewing ──人工──▶ Approved / Rejected
//
// 机审出错时进 StatusReviewing 而非放行：审核链路故障不能变成放行通道。
type Status string

const (
	StatusPending   Status = "pending"
	StatusReviewing Status = "reviewing"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
)

// IsPublic 判断该状态的内容能否出现在公开信息流中。
// 所有面向公众的查询都必须以此为准，不要在各处硬编码字符串比较。
func (s Status) IsPublic() bool { return s == StatusApproved }

// Valid 校验状态取值合法，防止外部输入写入非法状态。
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusReviewing, StatusApproved, StatusRejected:
		return true
	}
	return false
}

// TargetType 预留给日后扩展到评论、用户资料等其他内容类型。
type TargetType string

const TargetVideo TargetType = "video"

// Source 标识判定来自机器还是人工。
type Source string

const (
	SourceMachine Source = "machine"
	SourceManual  Source = "manual"
)

// Record 是一条审核流水。
//
// 必须落库而不能只依赖日志：日志按容量轮转（当前仅保留数日），
// 而《网络信息内容生态治理规定》要求平台留存相关处置记录，
// 涉及安全事件的记录留存期以月计。流水表是可靠的那一份。
type Record struct {
	ID         uint64     `gorm:"primaryKey" json:"id"`
	TargetType TargetType `gorm:"size:20;not null;index:idx_audit_target" json:"target_type"`
	TargetID   uint64     `gorm:"not null;index:idx_audit_target" json:"target_id"`
	// FromStatus 与 ToStatus 记录状态迁移，便于还原整条处置链路。
	FromStatus Status `gorm:"size:20;not null" json:"from_status"`
	ToStatus   Status `gorm:"size:20;not null" json:"to_status"`
	Source     Source `gorm:"size:20;not null" json:"source"`
	// Moderator 是做出判定的实现名或人工审核员账号，便于追溯是哪一版规则。
	Moderator string `gorm:"size:64;not null" json:"moderator"`
	// OperatorID 仅人工审核时有值。
	OperatorID uint64 `gorm:"not null;default:0" json:"operator_id"`
	Label      string `gorm:"size:64" json:"label"`
	// Detail 含命中词等内部依据，只在管理端展示，绝不返回给内容发布者。
	Detail    string    `gorm:"size:1000" json:"detail"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Record) TableName() string { return "audit_records" }

// RejectReasonForUser 是返回给发布者的统一措辞。
//
// 刻意不回显命中的词或具体类别：一旦回显，发布者就能通过反复修改试探
// 出词库边界，审核规则会被逐步摸清。这与错误响应不外泄内部细节同理。
const RejectReasonForUser = "内容未通过审核"

// PendingHintForUser 是内容待审时给发布者的提示。
const PendingHintForUser = "已提交，审核通过后将出现在信息流中"
