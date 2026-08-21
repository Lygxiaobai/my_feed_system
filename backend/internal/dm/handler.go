package dm

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

func (h *Handler) Inbox(c *gin.Context) {
	result, err := h.service.Inbox(c.GetUint64("account_id"))
	if err != nil {
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

func (h *Handler) Thread(c *gin.Context) {
	var req ThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.Thread(c.GetUint64("account_id"), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) MarkRead(c *gin.Context) {
	var req MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	if err := h.service.MarkRead(c.GetUint64("account_id"), req.PeerID); err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.Send(c.GetUint64("account_id"), req)
	if err != nil {
		writeErr(c, err)
		return
	}
	response.OK(c, result)
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPeerRequired):
		response.FailTip(c, http.StatusBadRequest, response.ParamMissing, "请选择聊天对象", err)
	case errors.Is(err, ErrSelf):
		response.FailTip(c, http.StatusBadRequest, response.ParamError, "不能给自己发私信", err)
	case errors.Is(err, ErrPeerMissing):
		response.FailTip(c, http.StatusNotFound, response.AccountNotFound, "对方账号不存在", err)
	case errors.Is(err, ErrEmptyBody):
		response.FailTip(c, http.StatusBadRequest, response.ParamMissing, "请输入要发送的内容", err)
	case errors.Is(err, ErrBodyTooLong):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "私信最多 500 个字", err)
	case errors.Is(err, ErrQuotaExceeded):
		response.FailTip(c, http.StatusForbidden, response.DMQuotaExceeded, "互相关注后才能继续发私信", err)
	default:
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
	}
}
