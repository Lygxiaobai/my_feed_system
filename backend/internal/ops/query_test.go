package ops

import "testing"

func TestNormalizeLogQuery(t *testing.T) {
	q, since, limit, err := normalizeLogQuery("", 0, 0)
	if err != nil || q != `{service="backend"}` || since != 60 || limit != 100 {
		t.Fatalf("defaults: %q %d %d %v", q, since, limit, err)
	}
	if _, _, _, err := normalizeLogQuery("backend", 60, 50); err == nil {
		t.Fatal("expected invalid query")
	}
	if _, _, _, err := normalizeLogQuery("{service=\"backend\"}", 60, 50); err != nil {
		t.Fatal(err)
	}
}
