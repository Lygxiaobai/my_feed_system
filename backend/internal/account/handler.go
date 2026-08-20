package account

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

// Handler 负责账号模块的 HTTP 接口编排。
type Handler struct {
	service *Service
}

// NewHandler 创建账号接口处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册无需登录即可访问的账号接口。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/email/sendCode", h.SendEmailCode)
	rg.POST("/email/verify", h.VerifyEmail)
	rg.POST("/findByID", h.FindByID)
	rg.POST("/findByUsername", h.FindByUsername)
}

// RegisterProtectedRoutes 注册需要登录后才能访问的账号接口。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.GET("/me", h.Me)
	rg.POST("/logout", h.Logout)
	rg.POST("/changePassword", h.ChangePassword)
	rg.POST("/rename", h.Rename)
}

// accountView 是账号对外暴露的公开字段，避免把实体直接序列化导致字段外泄。
func accountView(id uint64, username string) gin.H {
	return gin.H{"id": id, "username": username}
}

// Register 处理用户注册请求。
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	account, err := h.service.Register(req)
	if err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			response.Fail(c, http.StatusBadRequest, response.UsernameTaken, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"account": accountView(account.ID, account.Username)})
}

// Login 校验用户名密码并签发 JWT。
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	account, err := h.service.Login(req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredential) {
			// 不区分「账号不存在」与「密码错误」，避免被用于探测账号是否存在。
			response.Fail(c, http.StatusUnauthorized, response.CredentialWrong, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{
		"account": accountView(account.Account.ID, account.Account.Username),
		"token":   account.Token,
	})
}

// SendEmailCode 发送或建立邮箱验证码会话。
func (h *Handler) SendEmailCode(c *gin.Context) {
	var req SendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	if err := h.service.SendEmailCode(c.Request.Context(), req); err != nil {
		writeEmailError(c, err)
		return
	}
	response.OK(c, nil)
}

// VerifyEmail 用验证码登录或注册。
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.VerifyEmail(c.Request.Context(), req)
	if err != nil {
		writeEmailError(c, err)
		return
	}
	response.OK(c, gin.H{
		"account": accountView(result.Account.ID, result.Account.Username),
		"token":   result.Token,
		"created": result.Created,
	})
}

func writeEmailError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmailInvalid):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "邮箱格式不正确", err)
	case errors.Is(err, ErrEmailCodeInvalid):
		response.FailTip(c, http.StatusBadRequest, response.ParamError, "验证码不正确", err)
	case errors.Is(err, ErrEmailCooldown):
		response.FailTip(c, http.StatusTooManyRequests, response.RateLimited, "验证码发送过于频繁，请稍后再试", err)
	case errors.Is(err, ErrMailNotConfigured):
		response.FailTip(c, http.StatusServiceUnavailable, response.SystemError, "邮件服务未配置", err)
	case errors.Is(err, ErrMailSendFailed):
		response.FailTip(c, http.StatusServiceUnavailable, response.ThirdPartyError, "验证码发送失败，请稍后重试", err)
	case errors.Is(err, ErrEmailStoreMissing):
		response.Fail(c, http.StatusServiceUnavailable, response.CacheError, err)
	default:
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
	}
}

// FindByID 根据账号 ID 查询公开资料。
func (h *Handler) FindByID(c *gin.Context) {
	var req FindByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	account, err := h.service.FindByID(req)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"account": accountView(account.ID, account.Username)})
}

// FindByUsername 根据用户名查询公开资料。
func (h *Handler) FindByUsername(c *gin.Context) {
	var req FindByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	account, err := h.service.FindByUsername(req)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"account": accountView(account.ID, account.Username)})
}

// Me 返回当前 JWT 对应的账号信息。
func (h *Handler) Me(c *gin.Context) {
	response.OK(c, gin.H{
		"account": accountView(c.GetUint64("account_id"), c.GetString("account_username")),
	})
}

// Logout 清空数据库中的 token，实现服务端登出。
func (h *Handler) Logout(c *gin.Context) {
	accountID := c.GetUint64("account_id")
	if err := h.service.Logout(accountID); err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, nil)
}

// ChangePassword 校验旧密码并更新密码，同时使旧 token 失效。
func (h *Handler) ChangePassword(c *gin.Context) {
	accountID := c.GetUint64("account_id")
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	if err := h.service.ChangePassword(accountID, req); err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
			return
		}
		if errors.Is(err, ErrInvalidCredential) {
			response.FailTip(c, http.StatusUnauthorized, response.CredentialWrong, "原密码不正确", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, nil)
}

// Rename 修改用户名并重新签发 token，保证 JWT 中的用户名同步更新。
func (h *Handler) Rename(c *gin.Context) {
	accountID := c.GetUint64("account_id")
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.Rename(accountID, req)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			response.Fail(c, http.StatusNotFound, response.AccountNotFound, err)
			return
		}
		if errors.Is(err, ErrUsernameTaken) {
			response.Fail(c, http.StatusBadRequest, response.UsernameTaken, err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{
		"account": accountView(result.Account.ID, result.Account.Username),
		"token":   result.Token,
	})
}
