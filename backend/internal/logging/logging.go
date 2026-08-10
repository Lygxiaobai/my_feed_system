// Package logging 提供全局结构化日志能力。
//
// 选型说明：使用标准库 log/slog 而非直接调用某个具体日志实现。
// 这与《阿里巴巴 Java 开发手册》日志规约第 1 条「不可直接使用日志系统的 API，
// 而应依赖门面」是同一个道理——slog 的 Logger 是门面，Handler 是可替换的实现，
// 日后要换成文件轮转、接入采集系统都不需要改动业务代码。
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type requestIDKey struct{}

// WithRequestID 把请求 ID 写入 context，日志与响应体共用同一个值，
// 用户报障时凭响应里的 requestId 即可在日志中精确定位到那一次请求。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext 读取请求 ID，不存在时返回空字符串。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// contextHandler 在每条日志上自动补上 context 里的 request_id。
// Go 官方 slog 文档建议把 context 传进日志调用、由 Handler 提取 trace 信息，
// 这样业务代码不需要在每个调用点手动带上 ID，也就不会漏。
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		record.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(name)}
}

// sensitiveKeys 命中这些键名的字段一律脱敏，落实日志规约第 14 条。
// 判断按小写子串匹配，覆盖 password、jwt_token、Authorization 等各种写法。
var sensitiveKeys = []string{"password", "passwd", "secret", "token", "authorization", "cookie"}

func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, k := range sensitiveKeys {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func redact(_ []string, attr slog.Attr) slog.Attr {
	if isSensitive(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	return attr
}

// ParseLevel 解析配置中的日志级别，无法识别时回退到 info。
func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Setup 构建全局 Logger 并设为 slog 默认实例。
//
// format 为 "json" 时输出结构化 JSON，便于被采集系统直接解析；
// 其余值输出人类可读的文本，仅建议本地开发使用。
// service 会作为固定字段附加在每条日志上，便于多服务混合采集时区分来源。
func Setup(level string, format string, service string) *slog.Logger {
	options := &slog.HandlerOptions{
		Level:       ParseLevel(level),
		ReplaceAttr: redact,
	}

	var base slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		base = slog.NewTextHandler(os.Stdout, options)
	} else {
		base = slog.NewJSONHandler(os.Stdout, options)
	}

	logger := slog.New(contextHandler{Handler: base}).With(slog.String("service", service))
	slog.SetDefault(logger)
	return logger
}
