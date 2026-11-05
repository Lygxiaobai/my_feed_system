package account

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func testPasskeyService(t *testing.T) *Service {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Account{}, &Identity{}, &PasskeyCredential{}); err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewServiceWithTokenCache(db, nil, strings.Repeat("k", 32))
	svc.SetPasskeyStore(NewPasskeySessionStore(rdb))
	return svc
}

func TestParsePasskeyOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		origin string
		rpid   string
		ok     bool
	}{
		{"https://lvmouren.indevs.in", "lvmouren.indevs.in", true},
		{"https://lvmouren.indevs.in:443", "lvmouren.indevs.in", true},
		{"http://localhost:5173", "localhost", true},
		{"http://lvmouren.indevs.in", "", false},
		{"https://124.221.158.195", "", false},
		{"http://124.221.158.195:5173", "", false},
		{"https://user@lvmouren.indevs.in", "", false},
		{"https://lvmouren.indevs.in/path", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		rpid, canonical, err := parsePasskeyOrigin(tc.origin)
		if tc.ok {
			if err != nil {
				t.Fatalf("origin %q: %v", tc.origin, err)
			}
			if rpid != tc.rpid {
				t.Fatalf("origin %q rpid=%q want %q", tc.origin, rpid, tc.rpid)
			}
			if !strings.HasPrefix(canonical, "http") {
				t.Fatalf("origin %q canonical=%q", tc.origin, canonical)
			}
			continue
		}
		if !errors.Is(err, ErrPasskeyOrigin) {
			t.Fatalf("origin %q err=%v", tc.origin, err)
		}
	}
}

func TestAccountHandleRoundTrip(t *testing.T) {
	t.Parallel()
	got, err := parseAccountHandle(accountHandle(42))
	if err != nil || got != 42 {
		t.Fatalf("handle= %d err=%v", got, err)
	}
	if _, err := parseAccountHandle([]byte{1, 2}); !errors.Is(err, ErrPasskeyFailed) {
		t.Fatalf("short handle: %v", err)
	}
}

func TestPasskeyName(t *testing.T) {
	t.Parallel()
	name, err := normalizePasskeyName("  这台电脑  ")
	if err != nil || name != "这台电脑" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	if _, err := normalizePasskeyName(strings.Repeat("啊", 33)); !errors.Is(err, ErrPasskeyName) {
		t.Fatalf("long name: %v", err)
	}
	if _, err := normalizePasskeyName("bad\nname"); !errors.Is(err, ErrPasskeyName) {
		t.Fatalf("control name: %v", err)
	}
}

func TestBeginPasskeyLoginStoresOneShotSession(t *testing.T) {
	svc := testPasskeyService(t)
	ctx := context.Background()
	origin := "https://lvmouren.indevs.in"

	result, err := svc.BeginPasskeyLogin(ctx, origin)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID == "" {
		t.Fatal("missing session")
	}
	raw, _ := json.Marshal(result.Options)
	if !strings.Contains(string(raw), `"publicKey"`) {
		t.Fatalf("options=%s", raw)
	}

	_, err = svc.FinishPasskeyLogin(ctx, origin, PasskeyFinishRequest{
		SessionID:  result.SessionID,
		Credential: json.RawMessage(`{"id":"x","type":"public-key","response":{}}`),
	})
	if !errors.Is(err, ErrPasskeyFailed) {
		t.Fatalf("first finish: %v", err)
	}
	_, err = svc.FinishPasskeyLogin(ctx, origin, PasskeyFinishRequest{
		SessionID:  result.SessionID,
		Credential: json.RawMessage(`{"id":"x","type":"public-key","response":{}}`),
	})
	if !errors.Is(err, ErrPasskeyFailed) {
		t.Fatalf("replay finish: %v", err)
	}
}

func TestPasskeyFailureCasesAreUniform(t *testing.T) {
	svc := testPasskeyService(t)
	ctx := context.Background()
	origin := "https://lvmouren.indevs.in"

	cases := []PasskeyFinishRequest{
		{SessionID: "missing", Credential: json.RawMessage(`{}`)},
		{SessionID: "", Credential: json.RawMessage(`{"id":"x"}`)},
	}
	for _, req := range cases {
		_, err := svc.FinishPasskeyLogin(ctx, origin, req)
		if !errors.Is(err, ErrPasskeyFailed) && !errors.Is(err, ErrPasskeyOrigin) {
			t.Fatalf("req=%+v err=%v", req, err)
		}
	}
}

func TestBeginPasskeyRegisterReturnsDiscoverableOptions(t *testing.T) {
	svc := testPasskeyService(t)
	acc, err := svc.Register(RegisterRequest{Username: "pkopt", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.BeginPasskeyRegister(context.Background(), "https://lvmouren.indevs.in", acc.ID, "电脑")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result.Options)
	body := string(raw)
	for _, want := range []string{`"publicKey"`, `"residentKey":"required"`, `"attestation":"none"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestPasskeyRegisterRequiresAccountAndCap(t *testing.T) {
	svc := testPasskeyService(t)
	ctx := context.Background()
	origin := "https://lvmouren.indevs.in"

	_, err := svc.BeginPasskeyRegister(ctx, origin, 99, "")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("unknown account: %v", err)
	}

	acc, err := svc.Register(RegisterRequest{Username: "pkuser", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < passkeyMaxPerAccount; i++ {
		if err := svc.repo.CreatePasskey(&PasskeyCredential{
			AccountID:    acc.ID,
			CredentialID: []byte{byte(i + 1)},
			RecordJSON:   []byte(`{"id":"AQ"}`),
			Name:         "seed",
		}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = svc.BeginPasskeyRegister(ctx, origin, acc.ID, "")
	if !errors.Is(err, ErrPasskeyLimit) {
		t.Fatalf("limit: %v", err)
	}
}

func TestPasskeyListAndDeleteOwnership(t *testing.T) {
	svc := testPasskeyService(t)
	a, err := svc.Register(RegisterRequest{Username: "owner", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Register(RegisterRequest{Username: "other", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	record := &PasskeyCredential{
		AccountID:    a.ID,
		CredentialID: []byte{9, 8, 7},
		RecordJSON:   []byte(`{"id":"CQgH"}`),
		Name:         "本机",
	}
	if err := svc.repo.CreatePasskey(record); err != nil {
		t.Fatal(err)
	}

	items, err := svc.ListPasskeys(a.ID)
	if err != nil || len(items) != 1 || items[0].Name != "本机" {
		t.Fatalf("list owner: %+v err=%v", items, err)
	}
	if err := svc.DeletePasskey(b.ID, record.ID); !errors.Is(err, ErrPasskeyNotFound) {
		t.Fatalf("other delete: %v", err)
	}
	if err := svc.DeletePasskey(a.ID, record.ID); err != nil {
		t.Fatal(err)
	}
	items, err = svc.ListPasskeys(a.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("after delete: %+v err=%v", items, err)
	}
}

func TestPasskeyUnavailableWithoutStore(t *testing.T) {
	svc := NewServiceWithTokenCache(nil, nil, strings.Repeat("k", 32))
	_, err := svc.BeginPasskeyLogin(context.Background(), "https://lvmouren.indevs.in")
	if !errors.Is(err, ErrPasskeyUnavailable) {
		t.Fatalf("err=%v", err)
	}
}
