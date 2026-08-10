// Package response 定义全部 HTTP 接口的统一响应结构与错误码。
//
// 设计要点（对应《阿里巴巴 Java 开发手册》错误码规约第 7 条）：
// 堆栈、error_message、error_code、user_tip 是互相关联但不可越俎代庖的四样东西。
// 因此本包的写出函数只接受「错误码 + 面向用户的提示」作为响应内容，
// 底层 error 只能经由 err 参数进入日志——调用方没有把 err.Error() 写进响应体的途径，
// 内部实现细节泄漏到公网这件事在 API 形状上就被杜绝了。
package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/logging"
)

// Body 是所有接口统一的响应结构。
//
//	code      —— 机器可读的五位错误码，成功为 00000
//	message   —— 面向用户的提示（user_tip），可直接展示，成功时为空
//	data      —— 业务数据，失败时为 null
//	requestId —— 本次请求的唯一标识，与日志中的 request_id 一致，用于溯源
type Body struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

// OK 写出成功响应。
func OK(c *gin.Context, data any) {
	OKWithStatus(c, http.StatusOK, data)
}

// OKWithStatus 写出成功响应并指定 HTTP 状态码，
// 用于 202 Accepted 这类需要保留传输层语义的异步接口。
func OKWithStatus(c *gin.Context, status int, data any) {
	c.JSON(status, Body{
		Code:      Success,
		Message:   "",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Fail 写出失败响应，提示文案取错误码的默认值。
// err 允许为 nil；非 nil 时只写入日志，不进入响应体。
func Fail(c *gin.Context, status int, code Code, err error) {
	write(c, status, code, code.UserTip(), err)
}

// FailTip 与 Fail 相同，但用自定义文案覆盖默认提示，
// 用于需要更具体地引导用户的场景（例如指出是哪个字段不合法）。
func FailTip(c *gin.Context, status int, code Code, userTip string, err error) {
	write(c, status, code, userTip, err)
}

// Abort 写出失败响应并终止后续处理链，供中间件使用。
func Abort(c *gin.Context, status int, code Code, err error) {
	write(c, status, code, code.UserTip(), err)
	c.Abort()
}

func write(c *gin.Context, status int, code Code, userTip string, err error) {
	logFailure(c, status, code, err)

	c.JSON(status, Body{
		Code:      code,
		Message:   userTip,
		Data:      nil,
		RequestID: requestID(c),
	})
}

// logFailure 记录失败详情。
//
// 级别划分依据日志规约第 12 条：用户输入参数错误用 warn 记录，
// error 只留给系统逻辑出错，避免 4xx 触发频繁告警。
// 记录内容依据第 9 条：同时包含案发现场信息（路由、方法、账号）与原始错误。
func logFailure(c *gin.Context, status int, code Code, err error) {
	level := slog.LevelWarn
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	}

	ctx := c.Request.Context()
	if !slog.Default().Enabled(ctx, level) {
		return
	}

	attrs := []slog.Attr{
		slog.String("code", string(code)),
		slog.String("code_source", code.Source()),
		slog.Int("status", status),
		slog.String("method", c.Request.Method),
		slog.String("route", route(c)),
	}
	if accountID := c.GetUint64("account_id"); accountID != 0 {
		attrs = append(attrs, slog.Uint64("account_id", accountID))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	slog.Default().LogAttrs(ctx, level, "request failed", attrs...)
}

func requestID(c *gin.Context) string {
	return logging.RequestIDFromContext(c.Request.Context())
}

// route 优先返回路由模板而非真实路径，避免 /video/1、/video/2 这类
// 携带 ID 的路径把日志与指标的基数打爆。
func route(c *gin.Context) string {
	if full := c.FullPath(); full != "" {
		return full
	}
	return c.Request.URL.Path
}
