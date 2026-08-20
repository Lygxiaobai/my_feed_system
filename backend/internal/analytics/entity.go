package analytics

import (
	"strings"
	"time"
	"unicode"
)

const (
	maxEventsPerRequest = 20
	maxPageLen          = 200
	maxVisitorIDLen     = 64
	maxPropKeys         = 16
	maxPropStringLen    = 256
)

// 只接受产品约定过的事件名，避免任意字符串把日志检索面打散。
var allowedEvents = map[string]struct{}{
	"page_view":      {},
	"search":         {},
	"video_play":     {},
	"video_watch":    {},
	"video_like":     {},
	"video_unlike":   {},
	"comment_submit": {},
	"follow":         {},
	"unfollow":       {},
	"video_publish":  {},
	"login":          {},
	"register":       {},
	"logout":         {},
}

var sensitivePropKeys = []string{"password", "passwd", "secret", "token", "authorization", "cookie"}

// Event 是前端上报的一条产品行为。
type Event struct {
	Name       string         `json:"event"`
	Page       string         `json:"page"`
	ClientTS   int64          `json:"ts"`
	Properties map[string]any `json:"properties"`
}

// ReportRequest 一次可带多条事件，减少播放过程中的请求次数。
type ReportRequest struct {
	VisitorID string  `json:"visitor_id"`
	Events    []Event `json:"events"`
}

type acceptedEvent struct {
	Name       string
	Page       string
	VisitorID  string
	ClientTime time.Time
	Properties map[string]any
}

func isAllowedEvent(name string) bool {
	_, ok := allowedEvents[name]
	return ok
}

func sanitizeVisitorID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxVisitorIDLen {
		return ""
	}
	for _, r := range trimmed {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return ""
		}
	}
	return trimmed
}

func sanitizePage(raw string) string {
	page := strings.TrimSpace(raw)
	if page == "" {
		return "/"
	}
	if len(page) > maxPageLen {
		return page[:maxPageLen]
	}
	return page
}

func sanitizeProperties(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	out := make(map[string]any, min(len(raw), maxPropKeys))
	for key, value := range raw {
		if len(out) >= maxPropKeys {
			break
		}
		name := strings.TrimSpace(key)
		if name == "" || isSensitiveProp(name) {
			continue
		}
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			if len(text) > maxPropStringLen {
				text = text[:maxPropStringLen]
			}
			out[name] = text
		case bool:
			out[name] = typed
		case float64:
			out[name] = typed
		case int:
			out[name] = typed
		case int64:
			out[name] = typed
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isSensitiveProp(key string) bool {
	lower := strings.ToLower(key)
	for _, item := range sensitivePropKeys {
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}

func clientTime(ts int64) time.Time {
	if ts <= 0 {
		return time.Now()
	}
	parsed := time.UnixMilli(ts)
	now := time.Now()
	if parsed.After(now.Add(5 * time.Minute)) {
		return now
	}
	return parsed
}
