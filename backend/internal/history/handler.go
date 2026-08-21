package history

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
	"my_feed_system/internal/video"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Upsert(c *gin.Context) {
	var req UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.Upsert(c.GetUint64("account_id"), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.List(c.GetUint64("account_id"), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Progress(c *gin.Context) {
	var req ProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	items, err := h.service.Progress(c.GetUint64("account_id"), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, video.ErrVideoNotFound):
		response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
	case errors.Is(err, ErrInvalidProgress):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "播放进度不合法", err)
	case errors.Is(err, ErrInvalidStatus):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "历史状态不合法", err)
	case errors.Is(err, ErrInvalidCursor):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "分页参数不合法", err)
	case errors.Is(err, ErrTooManyIDs):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "一次查询的视频过多", err)
	default:
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
	}
}
