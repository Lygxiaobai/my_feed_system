package ops

import (
	"testing"

	"my_feed_system/internal/config"
)

func TestAllowedIsReviewerOnly(t *testing.T) {
	svc := NewService(config.OpsConfig{}, []uint64{1, 8})

	ok, err := svc.Allowed(1)
	if err != nil || !ok {
		t.Fatalf("reviewer 1: ok=%v err=%v", ok, err)
	}
	ok, err = svc.Allowed(8)
	if err != nil || !ok {
		t.Fatalf("reviewer 8: ok=%v err=%v", ok, err)
	}
	ok, err = svc.Allowed(11)
	if err != nil || ok {
		t.Fatalf("test-email account must be denied: ok=%v err=%v", ok, err)
	}
}
