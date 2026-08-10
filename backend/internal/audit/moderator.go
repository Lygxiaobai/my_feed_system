// Package audit 负责内容审核：状态机、判定结果与审核流水。
//
// 合规背景：《网络信息内容生态治理规定》要求平台建立「信息发布审核」制度，
// 并配备与服务规模相适应的专业人员。因此本模块的设计是
// 「机器初审 + 人工复审」两层，而不是纯机器判定——机审只负责分流，
// 拿不准的一律交给人。
package audit

import "context"

// Decision 是审核判定结果，刻意分三档而非两档。
//
// 只有 Pass/Block 两档时，任何拿不准的内容都会被迫二选一：
// 放行有合规风险，拦截则误伤正常用户。Review 档把这部分交给人工，
// 是机审能落地的前提。
type Decision string

const (
	// DecisionPass 明确无风险，可直接放行。
	DecisionPass Decision = "pass"
	// DecisionReview 无法确定，转人工复审。
	DecisionReview Decision = "review"
	// DecisionBlock 明确违规，直接拒绝。
	DecisionBlock Decision = "block"
)

// Result 是一次审核的判定结果。
//
// Label 与 Detail 只用于内部记录和人工复审展示，
// 绝不能返回给内容发布者——否则会被用来二分试探、反推词库规则。
type Result struct {
	Decision Decision
	// Label 是违规类别，例如 politics / porn / abuse / ad。
	Label string
	// Detail 是内部可读的判定依据，例如命中的词或第三方返回的原始标签。
	Detail string
}

// Pass 是可复用的通过结果。
func Pass() Result { return Result{Decision: DecisionPass} }

// Review 构造一个转人工的结果。
func Review(label string, detail string) Result {
	return Result{Decision: DecisionReview, Label: label, Detail: detail}
}

// Block 构造一个拒绝结果。
func Block(label string, detail string) Result {
	return Result{Decision: DecisionBlock, Label: label, Detail: detail}
}

// Moderator 是审核能力的抽象，也是接入云厂商的唯一扩展点。
//
// 当前只有基于本地词库的实现；日后接腾讯云内容安全（tms/ims/vm）时，
// 新增一个实现并在装配处替换即可，业务侧的状态机、流水和查询过滤都不用动。
// 组合多个实现（先本地后云端）也只需要一个装饰器，不影响调用方。
type Moderator interface {
	// ModerateText 审核标题、描述等文本内容。
	ModerateText(ctx context.Context, text string) (Result, error)
	// ModerateMedia 审核视频与封面。url 是可被审核服务访问的地址。
	ModerateMedia(ctx context.Context, playURL string, coverURL string) (Result, error)
	// Name 返回实现名，写入审核流水便于日后追溯是哪一版规则做的判定。
	Name() string
}
