package audit

import (
	"bufio"
	"context"
	"os"
	"strings"
	"sync"
	"unicode"
)

// MediaPolicy 决定在没有接入媒体审核能力时，视频与封面如何处置。
type MediaPolicy string

const (
	// MediaPolicyReview 转人工复审。合规上的正确默认值：
	// 没有能力审就不能假装审过了。
	MediaPolicyReview MediaPolicy = "review"
	// MediaPolicyPass 直接放行。仅供本地开发使用，生产环境不要用。
	MediaPolicyPass MediaPolicy = "pass"
)

// KeywordModerator 是基于本地词库的文本审核实现。
//
// 定位是「第一道闸」而不是完整方案：它零成本、毫秒级，
// 能拦掉最明显的违规内容，也避免把明显违规的文本再发给第三方。
// 真正的语义级判断要等接入云厂商。
type KeywordModerator struct {
	mu sync.RWMutex
	// blockWords 命中即拒绝。
	blockWords []string
	// reviewWords 命中转人工。
	reviewWords []string
	mediaPolicy MediaPolicy
}

// NewKeywordModerator 创建词库审核器。词库为空时文本一律放行，
// 此时整体仍受 mediaPolicy 约束，不会出现「完全无审核」的状态。
func NewKeywordModerator(blockWords []string, reviewWords []string, mediaPolicy MediaPolicy) *KeywordModerator {
	if mediaPolicy != MediaPolicyPass {
		mediaPolicy = MediaPolicyReview
	}
	return &KeywordModerator{
		blockWords:  normalizeWords(blockWords),
		reviewWords: normalizeWords(reviewWords),
		mediaPolicy: mediaPolicy,
	}
}

func (m *KeywordModerator) Name() string { return "keyword-local" }

// ModerateText 先归一化再匹配，拒绝优先于转人工。
func (m *KeywordModerator) ModerateText(_ context.Context, text string) (Result, error) {
	normalized := normalizeText(text)
	if normalized == "" {
		return Pass(), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先查拒绝词：同一段文本同时命中两类时，严格的那一档优先。
	if hit := firstHit(normalized, m.blockWords); hit != "" {
		return Block("keyword", "命中拒绝词: "+hit), nil
	}
	if hit := firstHit(normalized, m.reviewWords); hit != "" {
		return Review("keyword", "命中待审词: "+hit), nil
	}
	return Pass(), nil
}

// ModerateMedia 本地没有任何图像与视频识别能力，按配置的策略处置。
func (m *KeywordModerator) ModerateMedia(_ context.Context, _ string, _ string) (Result, error) {
	m.mu.RLock()
	policy := m.mediaPolicy
	m.mu.RUnlock()

	if policy == MediaPolicyPass {
		return Pass(), nil
	}
	return Review("media", "未接入媒体审核能力，转人工复审"), nil
}

// Reload 热更新词库，便于运营在不重启服务的情况下补充规则。
func (m *KeywordModerator) Reload(blockWords []string, reviewWords []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blockWords = normalizeWords(blockWords)
	m.reviewWords = normalizeWords(reviewWords)
}

func firstHit(normalized string, words []string) string {
	for _, w := range words {
		if strings.Contains(normalized, w) {
			return w
		}
	}
	return ""
}

func normalizeWords(words []string) []string {
	out := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, w := range words {
		n := normalizeText(w)
		if n == "" {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

// normalizeText 归一化文本，用于对抗常见的规避手法。
//
// 只做「把等价写法折叠成同一种」这一件事，不做语义理解：
//   - 全角转半角：Ａ→A，防止用全角字符绕过
//   - 统一小写：Abc→abc
//   - 去除空白与常见分隔符：「违 规」「违-规」→「违规」
//
// 注意这会带来一定误伤（例如正常句子去掉空格后偶然拼出敏感词），
// 所以词库里应尽量放足够长、歧义小的词，短词放 reviewWords 交人工判断。
func normalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		// 全角字符统一折算为半角
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == 0x3000: // 全角空格
			r = ' '
		}

		if unicode.IsSpace(r) {
			continue
		}
		// 丢弃常被插入用于分隔敏感词的标点
		if strings.ContainsRune(`-_.*~^|/\+=,，。、·—`, r) {
			continue
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

// LoadWordFile 从文件读取词库，每行一个词，忽略空行与 # 开头的注释。
//
// 词库本身属于敏感资产（会被用来反推规则），因此文件不进版本库，
// 与密钥同等对待。文件不存在时返回空列表而非报错：
// 未配置词库不应该阻止服务启动，媒体策略仍会兜住整体审核不为空。
func LoadWordFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words = append(words, line)
	}
	return words, scanner.Err()
}
