package wallet

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestVerifyStripeSignature(t *testing.T) {
	payload := []byte(`{"type":"checkout.session.completed"}`)
	secret := "whsec_test"
	ts := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", ts.Unix())))
	_, _ = mac.Write(payload)
	header := fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(mac.Sum(nil)))
	if err := verifyStripeSignature(payload, header, secret, ts, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := verifyStripeSignature(payload, header, secret, ts.Add(10*time.Minute), 5*time.Minute); err == nil {
		t.Fatal("stale timestamp must fail")
	}
	if err := verifyStripeSignature(payload, "t=1,v1=aaaa", secret, ts, 5*time.Minute); err == nil {
		t.Fatal("bad signature must fail")
	}
}

func TestWithOutTradeNo(t *testing.T) {
	got := withOutTradeNo("https://lvmouren.indevs.in/wallet", "WR1")
	if got != "https://lvmouren.indevs.in/wallet?out_trade_no=WR1" {
		t.Fatalf("got=%s", got)
	}
	canceled := withWalletReturn("https://lvmouren.indevs.in/wallet", "WR1", true)
	u, err := url.Parse(canceled)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("out_trade_no") != "WR1" || u.Query().Get("canceled") != "1" {
		t.Fatalf("canceled=%s", canceled)
	}
}
