package report

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

// Handler 暴露举报的用户面与处置面接口。
//
// 两类接口挂在同一个 /report 组下但权限不同：提交与「我的举报」对所有登录用户开放，
// 队列与处置只对审核员开放。
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterProtectedRoutes 注册需要登录的举报接口。
//
// 举报一律实名（登录态），不开放匿名举报：匿名会让「同一人只能举报一次」
// 这条约束失去着力点，举报量瞬间可以被刷到任意数字。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup, submitMiddleware ...gin.HandlerFunc) {
	rg.POST("/video", append(submitMiddleware, h.Submit)...)
	rg.POST("/mine", h.ListMine)
	rg.POST("/pending", h.ListPending)
	rg.POST("/handle", h.Handle)
}

func (h *Handler) Submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	row, err := h.service.Submit(c.GetUint64("account_id"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrContentNotFound):
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "内容不存在或已被删除", err)
		case errors.Is(err, ErrSelfReport):
			response.FailTip(c, http.StatusBadRequest, response.ParamError, "不能举报自己的内容", err)
		case errors.Is(err, ErrInvalidReason):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "请选择有效的举报理由", err)
		case errors.Is(err, ErrDetailRequired):
			response.FailTip(c, http.StatusBadRequest, response.ParamMissing, "选择「其他」时请填写具体说明", err)
		case errors.Is(err, ErrDetailTooLong):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "补充说明过长，请精简后再提交", err)
		case errors.Is(err, ErrAlreadyReported):
			response.FailTip(c, http.StatusConflict, response.DuplicatedRequest, "你已举报过该内容，我们会尽快处理", err)
		default:
			response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		}
		return
	}

	// 明确回执「已受理」而不是静默成功：举报人需要知道通知已经送达。
	response.OK(c, gin.H{
		"report": gin.H{
			"id":         row.ID,
			"status":     row.Status,
			"created_at": row.CreatedAt,
		},
	})
}

func (h *Handler) ListMine(c *gin.Context) {
	var req ListMineRequest
	// 允许空 body，用默认分页。
	_ = c.ShouldBindJSON(&req)

	rows, err := h.service.ListMine(c.GetUint64("account_id"), req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":          row.ID,
			"target_type": row.TargetType,
			"target_id":   row.TargetID,
			"reason":      row.Reason,
			"detail":      row.Detail,
			"status":      row.Status,
			"created_at":  row.CreatedAt,
			"handled_at":  row.HandledAt,
			// 刻意不返回 handler_id 与 handle_note：前者会暴露审核员身份，
			// 后者是内部判断依据，回显出去等于把处置尺度告诉举报方。
		})
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) ListPending(c *gin.Context) {
	var req ListPendingRequest
	_ = c.ShouldBindJSON(&req)

	items, err := h.service.ListPending(c.GetUint64("account_id"), req)
	if err != nil {
		if errors.Is(err, ErrNotReviewer) {
			response.Fail(c, http.StatusForbidden, response.AccessDenied, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) Handle(c *gin.Context) {
	var req HandleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.Handle(c.Request.Context(), c.GetUint64("account_id"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotReviewer):
			response.Fail(c, http.StatusForbidden, response.AccessDenied, err)
		case errors.Is(err, ErrInvalidAction):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "处置动作不合法", err)
		case errors.Is(err, ErrDetailTooLong):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "处置说明过长", err)
		case errors.Is(err, ErrNothingPending):
			response.FailTip(c, http.StatusConflict, response.DuplicatedRequest, "该内容没有待处理的举报", err)
		default:
			response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		}
		return
	}

	response.OK(c, gin.H{"resolved": result.ReportCount})
}
