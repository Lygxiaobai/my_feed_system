package db

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// slogGormLogger 把 GORM 的日志接进 slog。
//
// GORM 默认的 logger 直接往 stderr 写带 ANSI 颜色码的多行文本，
// 既破坏了全局统一的 JSON 格式，也会把 SQL 语句连同参数值一起打出来
// （日志规约第 14 条要求敏感信息脱敏，用户名、手机号这类参数不该进日志）。
// 这里只保留对排障真正有用的部分：耗时、影响行数、错误，以及慢查询告警。
type slogGormLogger struct {
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func newSlogGormLogger(slowThreshold time.Duration) gormlogger.Interface {
	return &slogGormLogger{level: gormlogger.Warn, slowThreshold: slowThreshold}
}

func (l *slogGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *slogGormLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Info {
		slog.InfoContext(ctx, "gorm", slog.String("detail", sprint(msg, args...)))
	}
}

func (l *slogGormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Warn {
		slog.WarnContext(ctx, "gorm", slog.String("detail", sprint(msg, args...)))
	}
}

func (l *slogGormLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Error {
		slog.ErrorContext(ctx, "gorm", slog.String("detail", sprint(msg, args...)))
	}
}

// Trace 在每条 SQL 执行后被调用。
// 注意这里刻意不记录 SQL 文本：fc() 返回的语句已内联参数值，属于敏感数据。
func (l *slogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		// ErrRecordNotFound 是正常的业务分支（查不到就是查不到），由上层转成 404，
		// 不应该在日志里表现为数据库错误，否则真正的故障会被淹没。
		_, rows := fc()
		slog.ErrorContext(ctx, "database query failed",
			slog.Int64("rows", rows),
			slog.Int64("elapsed_ms", elapsed.Milliseconds()),
			slog.String("error", err.Error()))
	case l.slowThreshold > 0 && elapsed >= l.slowThreshold:
		sql, rows := fc()
		slog.WarnContext(ctx, "slow sql",
			slog.Int64("rows", rows),
			slog.Int64("elapsed_ms", elapsed.Milliseconds()),
			// 慢查询必须知道是哪条语句才能优化，这里只取语句前缀且不含完整参数上下文。
			slog.String("sql_prefix", truncate(sql, 200)))
	}
}

func sprint(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	return msg + " " + trimArgs(args)
}

func trimArgs(args []any) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += truncate(toString(a), 120)
	}
	return truncate(out, 500)
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
