package video

import (
	"errors"
	"strings"
	"testing"
)

func TestShareCodeRoundTrip(t *testing.T) {
	ids := []uint64{1, 2, 3, 9, 10, 99, 1000, 65535, 1 << 20, mixMask - 1, mixMask}

	for _, id := range ids {
		code, err := EncodeShareCode(id)
		if err != nil {
			t.Fatalf("encode %d: %v", id, err)
		}
		if len(code) != shareCodeLength {
			t.Fatalf("code %q for id %d: got length %d, want %d", code, id, len(code), shareCodeLength)
		}
		got, err := DecodeShareCode(code)
		if err != nil {
			t.Fatalf("decode %q (id %d): %v", code, id, err)
		}
		if got != id {
			t.Fatalf("round trip id %d: got %d via %q", id, got, code)
		}
	}
}

// 相邻 ID 必须产生看起来不相邻的口令，否则口令等于把自增主键直接暴露出来。
func TestShareCodeAdjacentIDsDiffer(t *testing.T) {
	a, err := EncodeShareCode(1)
	if err != nil {
		t.Fatalf("encode 1: %v", err)
	}
	b, err := EncodeShareCode(2)
	if err != nil {
		t.Fatalf("encode 2: %v", err)
	}

	shared := 0
	for i := 0; i < shareCodePayloadLen; i++ {
		if a[i] == b[i] {
			shared++
		}
	}
	if shared > 3 {
		t.Fatalf("codes %q and %q share %d of %d payload chars, mixing is too weak", a, b, shared, shareCodePayloadLen)
	}
}

func TestEncodeShareCodeRejectsOutOfRange(t *testing.T) {
	for _, id := range []uint64{0, mixMask + 1} {
		if _, err := EncodeShareCode(id); !errors.Is(err, ErrShareCodeUnsupported) {
			t.Fatalf("encode %d: got %v, want ErrShareCodeUnsupported", id, err)
		}
	}
}

// 截断的口令必须被拒绝，而不是解析成另一个视频。
func TestDecodeShareCodeRejectsTruncated(t *testing.T) {
	code, err := EncodeShareCode(12345)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	for _, bad := range []string{code[:shareCodeLength-1], code[1:], code + "Z", ""} {
		if _, err := DecodeShareCode(bad); !errors.Is(err, ErrInvalidShareCode) {
			t.Fatalf("decode %q: got %v, want ErrInvalidShareCode", bad, err)
		}
	}
}

// 改动任意一个字符都应该被校验位挡下。
func TestDecodeShareCodeRejectsSingleCharCorruption(t *testing.T) {
	const id = uint64(4242)
	code, err := EncodeShareCode(id)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	accepted := 0
	for i := 0; i < shareCodeLength; i++ {
		for _, c := range []byte(shareAlphabet) {
			if code[i] == c {
				continue
			}
			mutated := code[:i] + string(c) + code[i+1:]
			if got, err := DecodeShareCode(mutated); err == nil && got != id {
				accepted++
			}
		}
	}
	if accepted != 0 {
		t.Fatalf("single-char corruption accepted %d times, checksum is not catching it", accepted)
	}
}

// 相邻字符互换应当被发现，已知盲点是二者恰好相差 16 的情况（见 shareChecksum 注释）。
func TestDecodeShareCodeRejectsTransposition(t *testing.T) {
	code, err := EncodeShareCode(987654)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	for i := 0; i < shareCodePayloadLen-1; i++ {
		a, _ := shareCharValue(code[i])
		b, _ := shareCharValue(code[i+1])
		if a == b || (a+16)&31 == b {
			continue
		}
		swapped := code[:i] + string(code[i+1]) + string(code[i]) + code[i+2:]
		if _, err := DecodeShareCode(swapped); err == nil {
			t.Fatalf("transposition at %d (%q) was accepted", i, swapped)
		}
	}
}

func TestDecodeShareCodeNormalizesConfusableChars(t *testing.T) {
	code, err := EncodeShareCode(777)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Crockford 约定：小写等价大写，I/L 读作 1，O 读作 0。
	variants := []string{
		strings.ToLower(code),
		strings.ReplaceAll(code, "1", "I"),
		strings.ReplaceAll(code, "0", "O"),
	}
	for _, v := range variants {
		got, err := DecodeShareCode(v)
		if err != nil {
			t.Fatalf("decode variant %q of %q: %v", v, code, err)
		}
		if got != 777 {
			t.Fatalf("decode variant %q: got %d, want 777", v, got)
		}
	}
}

func TestExtractShareCode(t *testing.T) {
	const id = uint64(4242)
	code, err := EncodeShareCode(id)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	cases := map[string]string{
		"裸口令":  code,
		"带空白":  "  " + code + "\n",
		"完整文案": code + ":/ 复制这段内容，打开站点粘贴到搜索框即可观看\n【阿绿的作品】测试标题\nhttps://example.com/s/" + code,
		"仅链接":  "https://example.com/s/" + code,
		"链接混在句子里": "看看这个 https://example.com/s/" + code + " 挺好玩的",
	}

	for name, text := range cases {
		got, err := ExtractShareCode(text)
		if err != nil {
			t.Fatalf("%s: extract %q: %v", name, text, err)
		}
		if got != id {
			t.Fatalf("%s: got %d, want %d", name, got, id)
		}
	}
}

// 普通文本不能被误识别成口令，否则用户搜索时会被莫名跳转到无关视频。
func TestExtractShareCodeIgnoresPlainText(t *testing.T) {
	texts := []string{
		"今天天气不错",
		"search for something",
		"abcdefgh ijklmnop qrstuvwx 12345678 password baseball",
		"https://example.com/video/12",
	}

	for _, text := range texts {
		if got, err := ExtractShareCode(text); err == nil {
			t.Fatalf("plain text %q was resolved to video %d", text, got)
		}
	}
}

func TestExtractShareCodeRejectsOversizedText(t *testing.T) {
	if _, err := ExtractShareCode(strings.Repeat("a", maxShareTextBytes+1)); !errors.Is(err, ErrShareTextTooLong) {
		t.Fatalf("got %v, want ErrShareTextTooLong", err)
	}
}
