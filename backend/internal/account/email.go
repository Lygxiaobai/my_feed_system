package account

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const otpHashSalt = "feed-otp-v1"

var (
	testEmailPattern = regexp.MustCompile(`^\d+@lmr\.com$`)
	emailPattern     = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	codePattern      = regexp.MustCompile(`^\d{6}$`)
)

func normalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func isTestEmail(email string) bool {
	return isTestEmailDomain(email, "lmr.com")
}

func isTestEmailDomain(email string, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		domain = "lmr.com"
	}
	if domain == "lmr.com" {
		return testEmailPattern.MatchString(email)
	}
	local, host, ok := strings.Cut(email, "@")
	if !ok || host != domain || local == "" {
		return false
	}
	for _, r := range local {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

func isValidCode(code string) bool {
	return codePattern.MatchString(code)
}

func hashOTP(email string, code string) string {
	sum := sha256.Sum256([]byte(otpHashSalt + "|" + email + "|" + code))
	return hex.EncodeToString(sum[:])
}

func usernameFromEmail(email string) string {
	local, _, _ := strings.Cut(email, "@")
	if isTestEmail(email) {
		return trimUsername("lmr_" + local)
	}

	var builder strings.Builder
	builder.WriteString("u_")
	for _, r := range local {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(unicode.ToLower(r))
		} else {
			builder.WriteByte('_')
		}
	}
	name := strings.Trim(builder.String(), "_")
	if name == "" || name == "u" {
		name = "u_user"
	}
	return trimUsername(name)
}

func trimUsername(name string) string {
	if len(name) > 48 {
		return name[:48]
	}
	return name
}

func uniqueUsername(base string, taken func(string) (bool, error)) (string, error) {
	candidate := base
	for i := 2; i < 1000; i++ {
		exists, err := taken(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = trimUsername(fmt.Sprintf("%s_%d", base, i))
	}
	return "", fmt.Errorf("allocate username for %s", base)
}
