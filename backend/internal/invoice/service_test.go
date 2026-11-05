package invoice

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"my_feed_system/internal/wallet"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&wallet.RechargeOrder{}, &wallet.Lot{}, &wallet.Ledger{}, &Profile{}, &Invoice{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testServices(t *testing.T, now time.Time) (*Service, *wallet.Service) {
	t.Helper()
	db := testDB(t)
	w := wallet.NewService(db)
	s := NewService(db, OrdersFromWallet(w))
	s.now = func() time.Time { return now }
	return s, w
}

func seedPaid(t *testing.T, db *gorm.DB, accountID uint64, no string, yuan, coins, bonus int64, now time.Time) {
	t.Helper()
	paid := now
	if err := db.Create(&wallet.RechargeOrder{
		AccountID:  accountID,
		OutTradeNo: no,
		Yuan:       yuan,
		Coins:      coins,
		Bonus:      bonus,
		Status:     wallet.OrderPaid,
		PayMethod:  wallet.PayMethodStripe,
		PaidAt:     &paid,
		ExpireAt:   now.Add(time.Hour),
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPersonalApplyIssuesReceipt(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, _ := testServices(t, now)
	seedPaid(t, s.db, 11, "WR11a", 6, 60, 0, now)

	row, err := s.Apply(11, ApplyRequest{
		OutTradeNo: "WR11a",
		Title:      "张三",
		Email:      "user@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != StatusIssued || row.Kind != KindPersonal || row.Yuan != 6 || row.Coins != 60 {
		t.Fatalf("issued = %+v", row)
	}
	if !strings.HasPrefix(row.InvoiceNo, "INV20260823") {
		t.Fatalf("invoice_no = %s", row.InvoiceNo)
	}
	if _, err := s.Apply(11, ApplyRequest{
		OutTradeNo: "WR11a",
		Title:      "张三",
		Email:      "user@example.com",
	}); !errors.Is(err, ErrAlreadyIssued) {
		t.Fatalf("second apply = %v", err)
	}
}

func TestCompanyApplyIsRejected(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, _ := testServices(t, now)
	seedPaid(t, s.db, 12, "WR12a", 68, 680, 100, now)
	if _, err := s.Apply(12, ApplyRequest{
		OutTradeNo: "WR12a",
		Kind:       "company",
		Title:      "示例科技有限公司",
		TaxID:      "91110000MA01234567",
		Email:      "fin@example.com",
	}); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("company apply = %v", err)
	}
}

func TestApplyRequiresOwnedPaidOrder(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, _ := testServices(t, now)
	if err := s.db.Create(&wallet.RechargeOrder{
		AccountID:  14,
		OutTradeNo: "WR14p",
		Yuan:       6,
		Coins:      60,
		Status:     wallet.OrderPending,
		ExpireAt:   now.Add(time.Hour),
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	req := ApplyRequest{OutTradeNo: "WR14p", Title: "李四", Email: "a@b.com"}
	if _, err := s.Apply(14, req); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("pending = %v", err)
	}
	seedPaid(t, s.db, 14, "WR14a", 6, 60, 0, now)
	if _, err := s.Apply(99, ApplyRequest{OutTradeNo: "WR14a", Title: "李四", Email: "a@b.com"}); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("other account = %v", err)
	}
}

func TestEligibleSkipsIssued(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, _ := testServices(t, now)
	seedPaid(t, s.db, 15, "WR15a", 6, 60, 0, now)
	seedPaid(t, s.db, 15, "WR15c", 68, 680, 100, now)
	if _, err := s.Apply(15, ApplyRequest{OutTradeNo: "WR15a", Title: "王五", Email: "a@b.com"}); err != nil {
		t.Fatal(err)
	}
	items, err := s.Eligible(15, ListRequest{})
	if err != nil || len(items) != 1 || items[0].OutTradeNo != "WR15c" {
		t.Fatalf("eligible = %+v err=%v", items, err)
	}
}

func TestApplyDoesNotCreditWallet(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, w := testServices(t, now)
	seedPaid(t, s.db, 16, "WR16a", 6, 60, 0, now)
	if _, err := s.Apply(16, ApplyRequest{OutTradeNo: "WR16a", Title: "赵六", Email: "a@b.com"}); err != nil {
		t.Fatal(err)
	}
	sum, err := w.Summary(16)
	if err != nil || sum.AvailableCoins != 0 {
		t.Fatalf("apply must not grant coins: %+v err=%v", sum, err)
	}
}

func TestGetHidesOthers(t *testing.T) {
	now := time.Date(2026, 8, 23, 21, 0, 0, 0, shanghai())
	s, _ := testServices(t, now)
	seedPaid(t, s.db, 18, "WR18a", 6, 60, 0, now)
	row, err := s.Apply(18, ApplyRequest{OutTradeNo: "WR18a", Title: "孙八", Email: "a@b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(19, row.InvoiceNo); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("other get = %v", err)
	}
}
