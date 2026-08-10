// Package requestid 为每个请求分配唯一标识，贯穿响应体、响应头与日志。
package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/logging"
)

// HeaderName 是本服务对外使用的请求 ID 响应头。
const HeaderName = "X-Request-Id"

// traceparentHeader 是 W3C Trace Context 定义的标准链路头，格式为：
//
//	00-<32 位 trace-id>-<16 位 span-id>-<2 位 flags>
//
// 若上游（网关、其他服务）已经带了该头，则复用其中的 trace-id 作为本次请求 ID，
// 这样同一次调用在多个服务的日志里能串起来；日后接入 APM 也不必改协议。
const traceparentHeader = "traceparent"

var traceparentPattern = regexp.MustCompile(`^[0-9a-f]{2}-([0-9a-f]{32})-[0-9a-f]{16}-[0-9a-f]{2}$`)

// New 返回请求 ID 中间件。
func New() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := fromUpstream(c)
		if id == "" {
			id = generate()
		}

		c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), id))
		// 提前写响应头，保证即使后续 panic 被 Recovery 捕获，调用方仍拿得到 ID。
		c.Header(HeaderName, id)
		c.Next()
	}
}

// fromUpstream 依次尝试从 traceparent 与 X-Request-Id 复用上游的标识。
func fromUpstream(c *gin.Context) string {
	if matches := traceparentPattern.FindStringSubmatch(c.GetHeader(traceparentHeader)); matches != nil {
		return matches[1]
	}
	if existing := c.GetHeader(HeaderName); isSafeID(existing) {
		return existing
	}
	return ""
}

// isSafeID 校验上游传入的 ID，避免把任意长度或含控制字符的内容写进日志与响应头。
func isSafeID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		isAllowed := (ch >= '0' && ch <= '9') ||
			(ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			ch == '-' || ch == '_'
		if !isAllowed {
			return false
		}
	}
	return true
}

// generate 生成 32 位十六进制 ID，与 W3C trace-id 的长度保持一致。
func generate() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// 随机源不可用时不应让请求失败，退化为固定前缀仍可保证日志有值可查。
		return "unknown-request-id"
	}
	return hex.EncodeToString(raw[:])
}
