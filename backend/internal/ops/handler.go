package ops

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"my_feed_system/internal/account"
	jwtmiddleware "my_feed_system/internal/middleware/jwt"
	"my_feed_system/internal/response"
)

const grafanaPath = "/grafana/d/feed-overview?kiosk=1&theme=dark"

type Handler struct {
	service    *Service
	db         *gorm.DB
	tokenCache *account.TokenCache
	jwtSecret  string
}

func NewHandler(service *Service, db *gorm.DB, tokenCache *account.TokenCache, jwtSecret string) *Handler {
	return &Handler{service: service, db: db, tokenCache: tokenCache, jwtSecret: jwtSecret}
}

type logsRequest struct {
	Query        string `json:"query"`
	SinceMinutes int    `json:"since_minutes"`
	Limit        int    `json:"limit"`
}

// Gate 给 nginx auth_request 用：Bearer 或运维 cookie 均可。
func (h *Handler) Gate(c *gin.Context) {
	token := jwtmiddleware.BearerToken(c)
	if token == "" {
		token = jwtmiddleware.CookieToken(c, jwtmiddleware.OpsCookieName)
	}
	if token == "" {
		response.Abort(c, http.StatusUnauthorized, response.LoginRequired, nil)
		return
	}
	if !jwtmiddleware.BindToken(c, h.db, h.tokenCache, h.jwtSecret, token) {
		return
	}
	ok, err := h.service.Allowed(c.GetUint64("account_id"))
	if err != nil {
		response.Abort(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	if !ok {
		response.Abort(c, http.StatusForbidden, response.AccessDenied, ErrOpsDenied)
		return
	}
	c.Status(http.StatusNoContent)
}

// Access 告诉前端当前账号能不能进运维台；允许时写入 Grafana 门禁 cookie。
func (h *Handler) Access(c *gin.Context) {
	accountID := c.GetUint64("account_id")
	ok, err := h.service.Allowed(accountID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}
	if ok {
		if token := jwtmiddleware.BearerToken(c); token != "" {
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(jwtmiddleware.OpsCookieName, token, 3600, "/", "", false, true)
		}
	} else {
		c.SetCookie(jwtmiddleware.OpsCookieName, "", -1, "/", "", false, true)
	}
	response.OK(c, gin.H{
		"allowed":      ok,
		"grafana_path": grafanaPath,
	})
}

func (h *Handler) Logs(c *gin.Context) {
	var req logsRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	result, err := h.service.QueryLogs(c.Request.Context(), c.GetUint64("account_id"), req.Query, req.SinceMinutes, req.Limit)
	if err != nil {
		writeOpsError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Metrics(c *gin.Context) {
	result, err := h.service.QueryMetrics(c.Request.Context(), c.GetUint64("account_id"))
	if err != nil {
		writeOpsError(c, err)
		return
	}
	response.OK(c, result)
}

func writeOpsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrOpsDenied):
		response.FailTip(c, http.StatusForbidden, response.AccessDenied, "没有查看运维信息的权限", err)
	case errors.Is(err, ErrQueryInvalid):
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "日志查询不合法", err)
	case errors.Is(err, ErrObservability):
		response.FailTip(c, http.StatusServiceUnavailable, response.ThirdPartyError, "观测数据暂时不可用", err)
	default:
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
	}
}
