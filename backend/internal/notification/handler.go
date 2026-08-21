package notification

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/list", h.List)
	rg.POST("/unreadCount", h.UnreadCount)
	rg.POST("/markRead", h.MarkRead)
	rg.POST("/markAllRead", h.MarkAllRead)
}

func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.service.List(c.GetUint64("account_id"), req)
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "筛选条件无效", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) UnreadCount(c *gin.Context) {
	result, err := h.service.UnreadCount(c.GetUint64("account_id"))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) MarkRead(c *gin.Context) {
	var req MarkReadRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.service.MarkRead(c.GetUint64("account_id"), req.IDs); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	var req MarkAllReadRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.service.MarkAllRead(c.GetUint64("account_id"), req.Kind); err != nil {
		if errors.Is(err, errInvalidCursor) {
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "筛选条件无效", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	response.OK(c, nil)
}
