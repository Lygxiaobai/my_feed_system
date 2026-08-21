package video

import (
	"errors"
	"regexp"
	"strings"
)

// 分享口令：由视频 ID 无状态推导，不落库。
//
// 不建表是刻意的：口令能从 ID 完全推导，建表只会多出「过期清理」和
// 「口令与内容不一致」两类需要维护的状态，而换不来任何能力。
//
// **口令不是访问控制。** 视频 ID 本来就能通过 /video/:id 公开访问，
// 下面的乘法混淆只是为了让口令不呈现自增规律（相邻 ID 的口令看起来不相邻），
// 不要把它当成安全边界，也不要因此放松 resolveShare 的审核状态过滤。
var (
	// ErrShareCodeUnsupported 表示视频 ID 超出可编码范围。
	ErrShareCodeUnsupported = errors.New("video id out of share code range")
	// ErrInvalidShareCode 表示口令格式非法、字符不在字母表内，或校验位不匹配。
	ErrInvalidShareCode = errors.New("invalid share code")
	// ErrShareTextTooLong 表示待解析文本超过长度上限。
	ErrShareTextTooLong = errors.New("share text too long")
	// ErrShareCodeNotFound 表示文本里没有可识别的口令。
	ErrShareCodeNotFound = errors.New("no share code found in text")
)

const (
	// shareAlphabet 是 Crockford Base32 字母表，去掉了易混淆的 I、L、O、U。
	// 去掉 U 还能顺带避免口令里拼出冒犯性单词。
	shareAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	// 7 个字符承载 35 位（7×5），末尾再加 1 位校验字符。
	shareCodePayloadLen = 7
	shareCodeLength     = shareCodePayloadLen + 1

	// mixMask 取 35 位，与 payload 位宽一致。
	mixMask = uint64(1)<<35 - 1
	// mixMultiplier 是 Knuth 乘法散列常数（2^32 × 黄金比例，取奇数）。
	// 奇数保证乘法在模 2^35 下是双射，因此存在唯一逆元、编解码可逆。
	mixMultiplier = uint64(0x9E3779B1)

	// maxShareTextBytes 限制待解析文本长度。粘贴内容是外部输入，
	// 不设上限会让扫描成本随请求体线性增长。
	maxShareTextBytes = 1024
)

// mixInverse 是 mixMultiplier 在模 2^35 下的乘法逆元。
//
// 用牛顿迭代在运行时求解而不是硬编码一个魔数：硬编码算错了要到
// 解码出错误视频时才会发现，而这里每次启动都由算法本身保证正确。
var mixInverse = func() uint64 {
	// 奇数 m 满足 m*m ≡ 1 (mod 8)，故 inv := m 已在模 2^3 下正确；
	// 每次迭代把正确位数翻倍，5 次足以覆盖 64 位。
	inv := mixMultiplier
	for range 5 {
		inv *= 2 - mixMultiplier*inv
	}
	return inv & mixMask
}()

var (
	// 口令后跟 ":/" 是分享文案里的显式标记。
	shareCodeMarkerRe = regexp.MustCompile(`([0-9A-Za-z]{8}):/`)
	// 分享链接形如 https://host/s/<code>。
	shareCodeURLRe = regexp.MustCompile(`/s/([0-9A-Za-z]{8})`)
	// 整段输入恰好是一个 8 字符 token 时，视为用户直接粘了口令。
	shareCodeBareRe = regexp.MustCompile(`^[0-9A-Za-z]{8}$`)
)

// EncodeShareCode 把视频 ID 编码成固定 8 字符的分享口令。
func EncodeShareCode(videoID uint64) (string, error) {
	if videoID == 0 || videoID > mixMask {
		return "", ErrShareCodeUnsupported
	}

	mixed := (videoID * mixMultiplier) & mixMask

	values := make([]uint64, shareCodeLength)
	for i := shareCodePayloadLen - 1; i >= 0; i-- {
		values[i] = mixed & 31
		mixed >>= 5
	}
	values[shareCodePayloadLen] = shareChecksum(values[:shareCodePayloadLen])

	var sb strings.Builder
	sb.Grow(shareCodeLength)
	for _, v := range values {
		sb.WriteByte(shareAlphabet[v])
	}
	return sb.String(), nil
}

// DecodeShareCode 把分享口令还原成视频 ID。
//
// 校验位不匹配时返回 ErrInvalidShareCode。这一位的价值在于：
// 粘贴内容被截断或改了一个字符时，若没有校验位就会静默解析成**另一个视频**，
// 用户会莫名其妙打开无关内容；有校验位才能明确告诉他口令无效。
func DecodeShareCode(code string) (uint64, error) {
	normalized := normalizeShareCode(code)
	if len(normalized) != shareCodeLength {
		return 0, ErrInvalidShareCode
	}

	values := make([]uint64, shareCodeLength)
	for i := 0; i < shareCodeLength; i++ {
		v, ok := shareCharValue(normalized[i])
		if !ok {
			return 0, ErrInvalidShareCode
		}
		values[i] = v
	}
	if values[shareCodePayloadLen] != shareChecksum(values[:shareCodePayloadLen]) {
		return 0, ErrInvalidShareCode
	}

	var mixed uint64
	for _, v := range values[:shareCodePayloadLen] {
		mixed = mixed<<5 | v
	}

	videoID := (mixed * mixInverse) & mixMask
	if videoID == 0 {
		return 0, ErrInvalidShareCode
	}
	return videoID, nil
}

// ExtractShareCode 从任意粘贴文本中识别出视频 ID。
//
// 只认三种形态，不做全文滑窗匹配：滑窗会在长文本里制造大量候选，
// 而单字符校验位只有 1/32 的拒绝率，几十个候选就足以撞出一个
// 「合法但完全无关」的视频。宁可少认，不可认错。
func ExtractShareCode(text string) (uint64, error) {
	if len(text) > maxShareTextBytes {
		return 0, ErrShareTextTooLong
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, ErrShareCodeNotFound
	}

	if shareCodeBareRe.MatchString(trimmed) {
		if videoID, err := DecodeShareCode(trimmed); err == nil {
			return videoID, nil
		}
		return 0, ErrShareCodeNotFound
	}

	for _, re := range []*regexp.Regexp{shareCodeURLRe, shareCodeMarkerRe} {
		for _, m := range re.FindAllStringSubmatch(trimmed, -1) {
			if videoID, err := DecodeShareCode(m[1]); err == nil {
				return videoID, nil
			}
		}
	}

	return 0, ErrShareCodeNotFound
}

// shareChecksum 用位置加权和生成校验值。
//
// 权重必须取奇数（1,3,5,…）：奇数与 32 互质，因而在模 32 下可逆，
// 任意单个字符出错都会改变校验值，**保证被发现**。
// 若用 1,2,3,… 这类含偶数的权重，位置 2 上恰好差 16 的错字会被整除掉，
// 校验值不变——那正是「口令错一个字符却打开了别的视频」的来源。
//
// 已知盲点：相邻两字符互换且二者恰好相差 16 时无法发现（约 6% 的互换）。
// 32 个符号、1 位校验字符的组合下，单字符错误与互换错误无法同时被完全覆盖
// （这也是 Crockford 原版改用模 37 的原因）。这里优先保证更常见的单字符错误。
func shareChecksum(values []uint64) uint64 {
	var sum uint64
	for i, v := range values {
		sum += uint64(2*i+1) * v
	}
	return sum & 31
}

// normalizeShareCode 统一大小写并按 Crockford 约定还原易混字符，
// 让用户手抄的 l/I/O 也能正确解析。
func normalizeShareCode(code string) string {
	var sb strings.Builder
	sb.Grow(len(code))
	for _, r := range code {
		switch r {
		case '-', ' ':
			// 分隔符只是书写习惯，忽略即可。
		case 'i', 'I', 'l', 'L':
			sb.WriteByte('1')
		case 'o', 'O':
			sb.WriteByte('0')
		default:
			if r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			}
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func shareCharValue(c byte) (uint64, bool) {
	idx := strings.IndexByte(shareAlphabet, c)
	if idx < 0 {
		return 0, false
	}
	return uint64(idx), true
}
