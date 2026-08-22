package account

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

func requestOrigin(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("Origin"))
}

func (h *Handler) BeginPasskeyRegister(c *gin.Context) {
	var req PasskeyRegisterBeginRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	result, err := h.service.BeginPasskeyRegister(c.Request.Context(), requestOrigin(c), c.GetUint64("account_id"), req.Name)
	if err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) FinishPasskeyRegister(c *gin.Context) {
	var req PasskeyFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	item, err := h.service.FinishPasskeyRegister(c.Request.Context(), requestOrigin(c), c.GetUint64("account_id"), req)
	if err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, gin.H{"passkey": item})
}

func (h *Handler) BeginPasskeyLogin(c *gin.Context) {
	result, err := h.service.BeginPasskeyLogin(c.Request.Context(), requestOrigin(c))
	if err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) FinishPasskeyLogin(c *gin.Context) {
	var req PasskeyFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	result, err := h.service.FinishPasskeyLogin(c.Request.Context(), requestOrigin(c), req)
	if err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, gin.H{
		"account": accountView(result.Account.ID, result.Account.Username),
		"token":   result.Token,
	})
}

func (h *Handler) ListPasskeys(c *gin.Context) {
	items, err := h.service.ListPasskeys(c.GetUint64("account_id"))
	if err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) DeletePasskey(c *gin.Context) {
	var req PasskeyDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	if err := h.service.DeletePasskey(c.GetUint64("account_id"), req.ID); err != nil {
		writePasskeyError(c, err)
		return
	}
	response.OK(c, nil)
}

func writePasskeyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPasskeyOrigin):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "当前页面无法使用通行密钥，请通过 HTTPS 域名访问", err)
	case errors.Is(err, ErrPasskeyFailed):
		response.FailTip(c, http.StatusUnauthorized, response.CredentialWrong, "通行密钥验证失败，请重试", err)
	case errors.Is(err, ErrPasskeyLimit):
		response.FailTip(c, http.StatusBadRequest, response.DuplicatedRequest, "通行密钥数量已达上限", err)
	case errors.Is(err, ErrPasskeyName):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "通行密钥名称不合法", err)
	case errors.Is(err, ErrPasskeyNotFound):
		response.Fail(c, http.StatusNotFound, response.ResourceNotFound, err)
	case errors.Is(err, ErrAccountNotFound):
		response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
	case errors.Is(err, ErrPasskeyUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CacheError, err)
	default:
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
	}
}
