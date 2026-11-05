package admin

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"my_feed_system/internal/account"
	"my_feed_system/internal/invoice"
	"my_feed_system/internal/report"
	"my_feed_system/internal/storage/storagetest"
	"my_feed_system/internal/video"
	"my_feed_system/internal/wallet"
)

func openFinanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin_finance?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	err = db.AutoMigrate(
		&account.Account{},
		&video.Video{},
		&report.Report{},
		&invoice.Invoice{},
		&wallet.Lot{},
		&wallet.RechargeOrder{},
		&wallet.PlatformEntry{},
	)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newFinanceAdmin(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := openFinanceDB(t)
	reports := report.NewService(db, nil, []uint64{1})
	accounts := account.NewService(db, "test-secret-at-least-32-bytes-long!!")
	videos := video.NewService(db, nil, storagetest.New())
	invoices := invoice.NewService(db, nil)
	wallets := wallet.NewService(db)
	return NewService(reports, videos, accounts, invoices, wallets), db
}

func TestFinanceBoardsReadOnly(t *testing.T) {
	svc, db := newFinanceAdmin(t)
	now := time.Now().UTC()
	soon := now.Add(24 * time.Hour)
	expired := now.Add(-time.Hour)

	reviewer := account.Account{Username: "reviewer"}
	alice := account.Account{Username: "alice"}
	bob := account.Account{Username: "bob"}
	if err := db.Create(&reviewer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&bob).Error; err != nil {
		t.Fatal(err)
	}
	if reviewer.ID != 1 {
		t.Fatalf("reviewer id = %d, want 1", reviewer.ID)
	}

	issued := now
	if err := db.Create(&invoice.Invoice{
		InvoiceNo:  "INV20260823TEST01",
		AccountID:  alice.ID,
		OutTradeNo: "R202608230001",
		Yuan:       6,
		Coins:      60,
		Bonus:      0,
		PayMethod:  wallet.PayMethodStripe,
		PaidAt:     now,
		Kind:       invoice.KindPersonal,
		Title:      "张三",
		Email:      "alice@example.com",
		Status:     invoice.StatusIssued,
		IssuedAt:   &issued,
	}).Error; err != nil {
		t.Fatal(err)
	}

	paidAt := now
	if err := db.Create(&wallet.RechargeOrder{
		AccountID:     alice.ID,
		OutTradeNo:    "R202608230001",
		Yuan:          6,
		Coins:         60,
		Status:        wallet.OrderPaid,
		PayMethod:     wallet.PayMethodStripe,
		StripeSession: "cs_test_abc",
		PaidAt:        &paidAt,
		ExpireAt:      now.Add(wallet.OrderTTL),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&wallet.RechargeOrder{
		AccountID:  alice.ID,
		OutTradeNo: "R202608230002",
		Yuan:       30,
		Coins:      300,
		Bonus:      30,
		Status:     wallet.OrderPending,
		PayMethod:  wallet.PayMethodStripe,
		ExpireAt:   now.Add(wallet.OrderTTL),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&[]wallet.Lot{
		{AccountID: alice.ID, Source: wallet.SourceRecharge, Remaining: 100, CreatedAt: now},
		{AccountID: alice.ID, Source: wallet.SourceCheckin, Remaining: 20, ExpireAt: &soon, CreatedAt: now},
		{AccountID: alice.ID, Source: wallet.SourceLottery, Remaining: 8, ExpireAt: &expired, CreatedAt: now},
		{AccountID: bob.ID, Source: wallet.SourceRegister, Remaining: 5, ExpireAt: &soon, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&wallet.PlatformEntry{Amount: 3, RefType: "tip", RefID: "1", CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListInvoices(alice.ID, ListInvoicesRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("ordinary list invoices: %v", err)
	}
	if _, err := svc.ListPayments(alice.ID, ListPaymentsRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("ordinary list payments: %v", err)
	}
	if _, err := svc.ListBalances(alice.ID, ListBalancesRequest{}); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("ordinary list balances: %v", err)
	}

	overview, err := svc.Overview(reviewer.ID, reviewer.Username)
	if err != nil {
		t.Fatal(err)
	}
	if overview.IssuedInvoices != 1 || overview.PaidYuan != 6 || overview.PaidOrders != 1 || overview.PendingOrders != 1 {
		t.Fatalf("overview payments %+v", overview)
	}
	if overview.AvailableCoins != 125 {
		t.Fatalf("overview coins = %d", overview.AvailableCoins)
	}

	invoices, err := svc.ListInvoices(reviewer.ID, ListInvoicesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if invoices.Summary.IssuedCount != 1 || invoices.Summary.YuanTotal != 6 || len(invoices.Invoices) != 1 {
		t.Fatalf("invoice board %+v", invoices)
	}
	if invoices.Invoices[0].Username != "alice" || invoices.Invoices[0].Email != "alice@example.com" {
		t.Fatalf("invoice row %+v", invoices.Invoices[0])
	}

	got, err := svc.GetInvoice(reviewer.ID, GetInvoiceRequest{InvoiceNo: "INV20260823TEST01"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Yuan != 6 || got.Title != "张三" {
		t.Fatalf("get invoice %+v", got)
	}

	pays, err := svc.ListPayments(reviewer.ID, ListPaymentsRequest{Status: wallet.OrderPaid})
	if err != nil {
		t.Fatal(err)
	}
	if pays.Summary.PaidCount != 1 || pays.Summary.PendingCount != 1 || pays.Summary.PlatformCutCoins != 3 {
		t.Fatalf("payment summary %+v", pays.Summary)
	}
	if len(pays.Orders) != 1 || pays.Orders[0].PayRef != "cs_test_abc" {
		t.Fatalf("paid orders %+v", pays.Orders)
	}

	if _, err := svc.ListPayments(reviewer.ID, ListPaymentsRequest{Status: "alipay"}); !errors.Is(err, ErrInvalidOrderStatus) {
		t.Fatalf("invalid status: %v", err)
	}

	balances, err := svc.ListBalances(reviewer.ID, ListBalancesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if balances.Summary.AccountsWithBalance != 2 || balances.Summary.AvailableCoins != 125 || balances.Summary.ExpiringSoonCoins != 25 {
		t.Fatalf("balance summary %+v", balances.Summary)
	}
	if len(balances.Balances) != 2 || balances.Balances[0].AccountID != alice.ID || balances.Balances[0].AvailableCoins != 120 {
		t.Fatalf("balance rows %+v", balances.Balances)
	}

	named, err := svc.ListBalances(reviewer.ID, ListBalancesRequest{Username: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	if len(named.Balances) != 1 || named.Balances[0].AvailableCoins != 5 {
		t.Fatalf("bob balance %+v", named.Balances)
	}

	detail, err := svc.GetBalance(reviewer.ID, GetBalanceRequest{ID: alice.ID})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.AvailableCoins != 120 || len(detail.Lots) != 2 {
		t.Fatalf("alice detail %+v lots=%d", detail.Summary, len(detail.Lots))
	}

	var leftover wallet.Lot
	if err := db.Where("account_id = ? AND source = ?", alice.ID, wallet.SourceLottery).First(&leftover).Error; err != nil {
		t.Fatal(err)
	}
	if leftover.Remaining != 8 {
		t.Fatalf("expired lot mutated: %+v", leftover)
	}
	var pending wallet.RechargeOrder
	if err := db.Where("out_trade_no = ?", "R202608230002").First(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending.Status != wallet.OrderPending {
		t.Fatalf("pending order mutated: %s", pending.Status)
	}
}
