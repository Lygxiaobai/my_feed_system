package account

import "testing"

func TestNormalizeAndTestEmail(t *testing.T) {
	if got := normalizeEmail("  ABC@LMR.COM "); got != "abc@lmr.com" {
		t.Fatalf("normalize = %q", got)
	}
	if !isTestEmail("123@lmr.com") {
		t.Fatal("expected test email")
	}
	if isTestEmail("ab@lmr.com") || isTestEmail("123@gmail.com") {
		t.Fatal("expected non-test email")
	}
	if !isValidCode("000000") || isValidCode("12345") || isValidCode("1234567") {
		t.Fatal("code validation")
	}
	if usernameFromEmail("88@lmr.com") != "lmr_88" {
		t.Fatalf("username = %q", usernameFromEmail("88@lmr.com"))
	}
}

func TestOTPHashIncludesEmail(t *testing.T) {
	a := hashOTP("a@lmr.com", "123456")
	b := hashOTP("b@lmr.com", "123456")
	if a == b || a == "" {
		t.Fatal("hash should depend on email")
	}
}

func TestIsTestEmailDomain(t *testing.T) {
	if !isTestEmailDomain("9@lmr.com", "") {
		t.Fatal("empty domain should default to lmr.com")
	}
	if !isTestEmailDomain("9@example.test", "example.test") {
		t.Fatal("custom domain")
	}
	if isTestEmailDomain("9@lmr.com", "example.test") {
		t.Fatal("wrong domain")
	}
}

func TestUniqueUsername(t *testing.T) {
	taken := map[string]bool{"u_alice": true, "u_alice_2": true}
	got, err := uniqueUsername("u_alice", func(name string) (bool, error) {
		return taken[name], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "u_alice_3" {
		t.Fatalf("got %q", got)
	}
}
