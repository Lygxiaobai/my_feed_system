package audit

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

// Handler 提供人工复审队列的接口。
//
// 这些接口只对配置中的审核员开放，属于管理面而非用户面，
// 因此挂在独立的 /audit 路由组下，且全部需要登录。
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterProtectedRoutes 注册需要登录的审核接口。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/listReviewing", h.ListReviewing)
	rg.POST("/review", h.Review)
	rg.POST("/history", h.History)
}

type listReviewingRequest struct {
	Limit    int    `json:"limit"`
	OffsetID uint64 `json:"offset_id"`
}

// ListReviewing 返回待人工复审的内容队列。
func (h *Handler) ListReviewing(c *gin.Context) {
	if !h.requireReviewer(c) {
		return
	}

	var req listReviewingRequest
	// 允许空 body，用默认分页参数。
	_ = c.ShouldBindJSON(&req)

	targets, err := h.service.ListReviewing(req.Limit, req.OffsetID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	items := make([]gin.H, 0, len(targets))
	for _, t := range targets {
		items = append(items, gin.H{
			"id":           t.ID,
			"author_id":    t.AuthorID,
			"username":     t.Username,
			"title":        t.Title,
			"description":  t.Description,
			"play_url":     t.PlayURL,
			"cover_url":    t.CoverURL,
			"audit_status": t.Status,
		})
	}
	response.OK(c, gin.H{"items": items})
}

type reviewRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
}

// Review 提交人工复审结论。
func (h *Handler) Review(c *gin.Context) {
	if !h.requireReviewer(c) {
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	status, err := h.service.ManualReview(c.Request.Context(), c.GetUint64("account_id"), req.VideoID, req.Approve, req.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrTargetNotFound):
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "内容不存在或已被删除", err)
		case errors.Is(err, ErrNotReviewable):
			response.FailTip(c, http.StatusConflict, response.DuplicatedRequest, "该内容已被处置，无需重复操作", err)
		case errors.Is(err, ErrNotReviewer):
			response.Fail(c, http.StatusForbidden, response.AccessDenied, err)
		default:
			response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		}
		return
	}

	response.OK(c, gin.H{"audit_status": status})
}

type historyRequest struct {
	VideoID uint64 `json:"video_id" binding:"required"`
}

// History 返回某个内容的完整处置链路，供追溯。
func (h *Handler) History(c *gin.Context) {
	if !h.requireReviewer(c) {
		return
	}

	var req historyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	records, err := h.service.History(req.VideoID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, gin.H{"records": records})
}

// requireReviewer 校验审核员身份，未通过时已写出响应，调用方直接返回即可。
func (h *Handler) requireReviewer(c *gin.Context) bool {
	if h.service.IsReviewer(c.GetUint64("account_id")) {
		return true
	}
	response.Fail(c, http.StatusForbidden, response.AccessDenied, nil)
	return false
}
