package invoice

import "testing"

func TestNormalizeHeaderPersonal(t *testing.T) {
	got, err := normalizeHeader(Header{Title: " 张 三 ", Email: "User@Example.COM"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindPersonal || got.Title != "张 三" || got.Email != "user@example.com" {
		t.Fatalf("got=%+v", got)
	}
}

func TestNormalizeHeaderRejectsCompany(t *testing.T) {
	_, err := normalizeHeader(Header{Kind: "company", Title: "示例科技有限公司", Email: "a@b.com"})
	if err != ErrInvalidKind {
		t.Fatalf("company header = %v", err)
	}
}

func TestNormalizeHeaderRejectsShortTitleAndBadEmail(t *testing.T) {
	if _, err := normalizeHeader(Header{Title: "甲", Email: "a@b.com"}); err != ErrInvalidTitle {
		t.Fatalf("short title: %v", err)
	}
	if _, err := normalizeHeader(Header{Title: "张三", Email: "not-an-email"}); err != ErrInvalidEmail {
		t.Fatalf("bad email: %v", err)
	}
}
