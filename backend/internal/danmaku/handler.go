package danmaku

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

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/list", h.List)
}

func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/send", h.Send)
}

func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	items, err := h.service.List(c.GetUint64("account_id"), req)
	if err != nil {
		if isVideoMissing(err) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"items": items})
}

func (h *Handler) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	item, err := h.service.Send(c.GetUint64("account_id"), c.GetString("account_username"), req)
	if err != nil {
		switch {
		case isVideoMissing(err):
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
		case errors.Is(err, ErrEmptyContent):
			response.FailTip(c, http.StatusBadRequest, response.ParamMissing, "请输入弹幕内容", err)
		case errors.Is(err, ErrContentTooLong):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "弹幕最多 50 个字", err)
		case errors.Is(err, ErrInvalidOffset):
			response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "播放进度不合法", err)
		default:
			response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		}
		return
	}

	response.OK(c, gin.H{"item": item})
}
