// Package accesslog 记录 HTTP 访问日志。
//
// 之所以不直接用 gin.Logger()：它的输出契约是 func(LogFormatterParams) string，
// 最终以 fmt.Fprint 写出一整行拼好的字符串，字段结构在那一步就丢失了，
// 无法交给 slog 分级、也无法被采集系统按字段查询。
// 这里在同样的位置（handler 前后）采集，但把字段原样交给 slog。
package accesslog

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Options 控制访问日志行为。
type Options struct {
	// SkipPaths 中的路径不记录访问日志。
	// 健康检查每十秒一次，长期占据日志的绝大多数行数，
	// 属于日志规约第 11 条所说「记录了也没人看」的无效日志。
	SkipPaths []string
	// SlowThreshold 超过该耗时的请求提升为 warn，便于发现慢接口。零值表示不启用。
	SlowThreshold time.Duration
}

// New 返回访问日志中间件。
func New(options Options) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(options.SkipPaths))
	for _, p := range options.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, skipped := skip[c.Request.URL.Path]; skipped {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		level := levelFor(status, latency, options.SlowThreshold)

		ctx := c.Request.Context()
		if !slog.Default().Enabled(ctx, level) {
			return
		}

		// 使用 LogAttrs 与类型化 Attr：Go 官方文档指出这一组合可以显著减少内存分配，
		// 访问日志是每请求必走的热路径，值得如此。
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", routeOf(c)),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("bytes", c.Writer.Size()),
		}
		if accountID := c.GetUint64("account_id"); accountID != 0 {
			attrs = append(attrs, slog.Uint64("account_id", accountID))
		}
		// gin 在处理链中收集到的错误（如 panic 恢复）一并带出，避免线索丢失。
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("gin_errors", c.Errors.String()))
		}

		slog.Default().LogAttrs(ctx, level, "http request", attrs...)
	}
}

// levelFor 依据日志规约第 12 条划分级别：
// 用户输入类错误（4xx）用 warn，error 只留给系统自身出错（5xx），避免 4xx 频繁告警。
func levelFor(status int, latency time.Duration, slowThreshold time.Duration) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case slowThreshold > 0 && latency >= slowThreshold:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// routeOf 返回路由模板而非真实路径，防止路径中的 ID 把日志基数打爆。
func routeOf(c *gin.Context) string {
	if full := c.FullPath(); full != "" {
		return full
	}
	// 未匹配到任何路由（404）时 FullPath 为空，此时记录原始路径才有排查价值。
	return c.Request.URL.Path
}
