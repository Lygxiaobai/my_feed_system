package notification

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// mentionRe 匹配评论里的 @用户名。
//
// 前缀排除字母数字下划线，避免把邮箱 local@domain 里的 domain 当成提及。
// 用户名字符集按账号实际能出现的来：字母、数字、下划线、连字符。
var mentionRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])@([\p{L}\p{N}_-]{1,64})`)

// ParseMentions 抽出评论中最多 maxMentions 个不重复的 @用户名，保持出现顺序。
func ParseMentions(content string) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
		if len(out) >= maxMentions {
			break
		}
	}
	return out
}

func clipText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
