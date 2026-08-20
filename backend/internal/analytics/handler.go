package analytics

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

// Handler 接收前端产品埋点并写成可检索的结构化日志。
type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/report", h.Report)
}

// Report 校验并接受一批事件。
// 不落库：产品分析先走 Loki 检索，避免为观测流量再开一张主数据表。
func (h *Handler) Report(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}
	if len(req.Events) == 0 {
		response.FailTip(c, http.StatusBadRequest, response.ParamMissing, "缺少上报事件", nil)
		return
	}
	if len(req.Events) > maxEventsPerRequest {
		response.FailTip(c, http.StatusBadRequest, response.ParamError, "一次上报事件过多", fmt.Errorf("events=%d", len(req.Events)))
		return
	}

	visitorID := sanitizeVisitorID(req.VisitorID)
	if visitorID == "" {
		response.FailTip(c, http.StatusBadRequest, response.ParamFormatError, "访客标识不合法", nil)
		return
	}

	accepted := make([]acceptedEvent, 0, len(req.Events))
	for _, item := range req.Events {
		name := item.Name
		if !isAllowedEvent(name) {
			response.FailTip(c, http.StatusBadRequest, response.ParamError, "存在不支持的事件", fmt.Errorf("event=%s", name))
			return
		}
		accepted = append(accepted, acceptedEvent{
			Name:       name,
			Page:       sanitizePage(item.Page),
			VisitorID:  visitorID,
			ClientTime: clientTime(item.ClientTS),
			Properties: sanitizeProperties(item.Properties),
		})
	}

	accountID := c.GetUint64("account_id")
	for _, item := range accepted {
		attrs := []slog.Attr{
			slog.String("event", item.Name),
			slog.String("visitor_id", item.VisitorID),
			slog.String("page", item.Page),
			slog.Time("client_ts", item.ClientTime),
		}
		if accountID != 0 {
			attrs = append(attrs, slog.Uint64("account_id", accountID))
		}
		if len(item.Properties) > 0 {
			attrs = append(attrs, slog.Any("properties", item.Properties))
		}
		slog.Default().LogAttrs(c.Request.Context(), slog.LevelInfo, "product_event", attrs...)
	}

	response.OK(c, gin.H{"accepted": len(accepted)})
}
