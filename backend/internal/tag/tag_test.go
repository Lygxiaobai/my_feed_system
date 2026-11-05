package tag

import (
	"reflect"
	"testing"
)

func TestListScanNil(t *testing.T) {
	var got List
	if err := got.Scan(nil); err != nil {
		t.Fatalf("null should scan: %v", err)
	}
	if got != nil {
		t.Fatalf("null should become empty, got %v", got)
	}
}

func TestMergeFIFO(t *testing.T) {
	got := Merge(nil, []string{"a", "b", "c", "d", "e", "f", "g", "h"})
	want := []string{"b", "c", "d", "e", "f", "g", "h"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFromHashtagsIgnoresBareTitle(t *testing.T) {
	if got := FromHashtags("#美食 探店", "周末"); !reflect.DeepEqual(got, []string{"美食"}) {
		t.Fatalf("hashtag %v", got)
	}
	if got := FromHashtags("没有话题的标题", ""); len(got) != 0 {
		t.Fatalf("bare title should not become a tag: %v", got)
	}
}

func TestForPublishKeepsExplicitOnly(t *testing.T) {
	got := ForPublish([]string{"旅行", "夜市"}, "#美食 探店", "#夜市 周末")
	want := []string{"旅行", "夜市"}
	if !reflect.DeepEqual([]string(got), want) {
		t.Fatalf("explicit should not merge leftover hashtags: %v", got)
	}
}

func TestInferUsesTitleAndHashtags(t *testing.T) {
	got := Infer("哆啦a懵测试", "哆啦a懵测试")
	if !reflect.DeepEqual(got, []string{"哆啦a懵测试"}) {
		t.Fatalf("bare title %v", got)
	}
	got = Infer("#美食 探店", "周末")
	if !reflect.DeepEqual(got, []string{"美食", "探店", "周末"}) {
		t.Fatalf("hashtag plus leftover %v", got)
	}
}

func TestForPublishFallsBackToInferred(t *testing.T) {
	got := ForPublish(nil, "#美食 探店", "#夜市 周末")
	want := []string{"美食", "夜市", "探店", "周末"}
	if !reflect.DeepEqual([]string(got), want) {
		t.Fatalf("empty choice should fall back to inferred: %v", got)
	}
	got = ForPublish(nil, "哆啦a懵测试", "")
	if !reflect.DeepEqual([]string(got), []string{"哆啦a懵测试"}) {
		t.Fatalf("bare title fallback %v", got)
	}
}

func TestNormalizeCapsAtSeven(t *testing.T) {
	got := Normalize([]string{"1", "2", "3", "4", "5", "6", "7", "8"})
	if !reflect.DeepEqual(got, []string{"1", "2", "3", "4", "5", "6", "7"}) {
		t.Fatalf("got %v", got)
	}
}
