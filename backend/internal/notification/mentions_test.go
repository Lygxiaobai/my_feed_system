package notification

import "testing"

func TestParseMentions(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "empty", content: "你好", want: nil},
		{name: "single", content: "看这个 @alice", want: []string{"alice"}},
		{name: "duplicate", content: "@Alice 和 @alice 都在", want: []string{"Alice"}},
		{name: "email-not-mention", content: "联系 user@example.com", want: nil},
		{name: "multiple", content: "@bob 回复 @carol 哈", want: []string{"bob", "carol"}},
		{name: "underscore", content: "hi @lmr_8801", want: []string{"lmr_8801"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseMentions(tc.content)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %#v want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestClipText(t *testing.T) {
	if clipText("  短  ", 80) != "短" {
		t.Fatalf("trim failed: %q", clipText("  短  ", 80))
	}
	long := "一二三四五六七八九十"
	got := clipText(long, 4)
	if got != "一二三四…" {
		t.Fatalf("clip = %q", got)
	}
}
