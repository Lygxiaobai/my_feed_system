package wallet

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/audit"
	"my_feed_system/internal/video"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&account.Account{},
		&account.Identity{},
		&video.Video{},
		&Lot{},
		&Ledger{},
		&DailyAction{},
		&RechargeOrder{},
		&TipRecord{},
		&PlatformEntry{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func testService(t *testing.T, now time.Time) *Service {
	t.Helper()
	s := NewService(testDB(t))
	s.now = func() time.Time { return now }
	return s
}

func TestSpendUsesSoonestExpiryThenRecharge(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	soon := now.Add(2 * time.Hour)
	later := now.Add(48 * time.Hour)

	if err := s.db.Create([]*Lot{
		{AccountID: 1, Source: SourceRegister, Remaining: 10, ExpireAt: &soon, CreatedAt: now},
		{AccountID: 1, Source: SourceTipIn, Remaining: 10, ExpireAt: &later, CreatedAt: now},
		{AccountID: 1, Source: SourceRecharge, Remaining: 20, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.spendInTx(tx, 1, 15, LedgerConsumeTip, "tip", "1")
	}); err != nil {
		t.Fatal(err)
	}

	var lots []Lot
	if err := s.db.Order("id").Find(&lots).Error; err != nil {
		t.Fatal(err)
	}
	if lots[0].Remaining != 0 || lots[1].Remaining != 5 || lots[2].Remaining != 20 {
		t.Fatalf("remaining = %+v", lots)
	}
}

func TestSplitTipRoundsAuthorDown(t *testing.T) {
	got, cut := splitTip(11)
	if got != 7 || cut != 4 {
		t.Fatalf("11 -> %d/%d", got, cut)
	}
	got, cut = splitTip(10)
	if got != 7 || cut != 3 {
		t.Fatalf("10 -> %d/%d", got, cut)
	}
}

func TestCheckinOncePerBeijingDay(t *testing.T) {
	now := time.Date(2026, 8, 20, 23, 30, 0, 0, shanghai())
	s := testService(t, now)
	s.drawCheckin = func() (int, error) { return 0, nil }
	if _, err := s.Checkin(3); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Checkin(3); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("second checkin = %v", err)
	}
	s.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := s.Checkin(3); err != nil {
		t.Fatalf("next day should work: %v", err)
	}
}

func TestCheckinWritesDrawnPrize(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, shanghai())
	s := testService(t, now)
	s.drawCheckin = func() (int, error) { return 950, nil } // 11 积分
	prize, err := s.Checkin(11)
	if err != nil || prize != 11 {
		t.Fatalf("prize=%d err=%v", prize, err)
	}
	var action DailyAction
	if err := s.db.Where("account_id = ? AND action = ?", 11, ActionCheckin).First(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.Prize != 11 || action.BizDate != "2026-08-20" {
		t.Fatalf("daily action = %+v", action)
	}
	sum, err := s.Summary(11)
	if err != nil || sum.AvailableCoins != 11 {
		t.Fatalf("summary=%+v err=%v", sum, err)
	}
}

func TestCheckinMonthListsBeijingDays(t *testing.T) {
	now := time.Date(2026, 8, 21, 9, 0, 0, 0, shanghai())
	s := testService(t, now)
	if err := s.db.Create([]*DailyAction{
		{AccountID: 12, Action: ActionCheckin, BizDate: "2026-07-31", Prize: 3, CreatedAt: now},
		{AccountID: 12, Action: ActionCheckin, BizDate: "2026-08-01", Prize: 7, CreatedAt: now},
		{AccountID: 12, Action: ActionCheckin, BizDate: "2026-08-21", Prize: 14, CreatedAt: now},
		{AccountID: 12, Action: ActionLottery, BizDate: "2026-08-02", Prize: 50, CreatedAt: now},
		{AccountID: 99, Action: ActionCheckin, BizDate: "2026-08-21", Prize: 1, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	out, err := s.CheckinMonth(12)
	if err != nil {
		t.Fatal(err)
	}
	if out.Year != 2026 || out.Month != 8 || out.Today != "2026-08-21" || !out.ClaimedToday {
		t.Fatalf("month header = %+v", out)
	}
	if len(out.Days) != 2 || out.Days[0] != (CheckinDay{BizDate: "2026-08-01", Coins: 7}) || out.Days[1] != (CheckinDay{BizDate: "2026-08-21", Coins: 14}) {
		t.Fatalf("days = %+v", out.Days)
	}
}

func TestLotteryWritesZeroLedger(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, shanghai())
	s := testService(t, now)
	s.draw = func() (int, error) { return 0, nil }
	result, err := s.Lottery(4)
	if err != nil || result.Coins != 0 || result.PrizeIndex != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	sum, err := s.Summary(4)
	if err != nil || sum.AvailableCoins != 0 {
		t.Fatalf("summary=%+v err=%v", sum, err)
	}
	var n int64
	if err := s.db.Model(&Ledger{}).Where("account_id = ? AND biz_type = ?", 4, LedgerGrantLottery).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("ledger count = %d", n)
	}
}

func TestRegisterGiftOnCreate(t *testing.T) {
	db := testDB(t)
	walletSvc := NewService(db)
	accSvc := account.NewService(db, "not-used-for-register-but-long-enough-secret")
	accSvc.SetCreatedHook(walletSvc.GrantRegisterGiftTx)
	acc, err := accSvc.Register(account.RegisterRequest{Username: "newuser", Password: "secret12"})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := walletSvc.Summary(acc.ID)
	if err != nil || sum.AvailableCoins != RegisterGiftCoins {
		t.Fatalf("gift = %+v err=%v", sum, err)
	}
	if err := walletSvc.db.Transaction(func(tx *gorm.DB) error {
		return walletSvc.GrantRegisterGiftTx(tx, acc.ID)
	}); err != nil {
		t.Fatal(err)
	}
	sum, _ = walletSvc.Summary(acc.ID)
	if sum.AvailableCoins != RegisterGiftCoins {
		t.Fatalf("gift must be once, got %d", sum.AvailableCoins)
	}
}

func TestTipRules(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	item := &video.Video{
		AuthorID:    9,
		Username:    "author",
		Title:       "v",
		PlayURL:     "/static/videos/a.mp4",
		AuditStatus: audit.StatusApproved,
	}
	if err := s.db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.grantInTx(s.db, 8, SourceRecharge, 100, LedgerGrantRecharge, "recharge", "seed"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Tip(8, "alice", TipRequest{VideoID: item.ID, Coins: 9}); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("min coins: %v", err)
	}
	if _, err := s.Tip(9, "author", TipRequest{VideoID: item.ID, Coins: 10}); !errors.Is(err, ErrTipSelf) {
		t.Fatalf("self tip: %v", err)
	}
	rec, err := s.Tip(8, "alice", TipRequest{VideoID: item.ID, Coins: 10})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Received != 7 || rec.Cut != 3 {
		t.Fatalf("cut = %+v", rec)
	}
	if _, err := s.Tip(8, "alice", TipRequest{VideoID: item.ID, Coins: 10}); !errors.Is(err, ErrTipTooFrequent) {
		t.Fatalf("debounce: %v", err)
	}
	author, err := s.Summary(9)
	if err != nil || author.AvailableCoins != 7 {
		t.Fatalf("author summary=%+v err=%v", author, err)
	}
	var cuts int64
	if err := s.db.Model(&PlatformEntry{}).Select("COALESCE(SUM(amount),0)").Scan(&cuts).Error; err != nil {
		t.Fatal(err)
	}
	if cuts != 3 {
		t.Fatalf("platform cut = %d", cuts)
	}
}

func TestExpireSweep(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	past := now.Add(-time.Minute)
	if err := s.db.Create(&Lot{AccountID: 2, Source: SourceRegister, Remaining: 10, ExpireAt: &past, CreatedAt: now.Add(-RegisterGiftTTL)}).Error; err != nil {
		t.Fatal(err)
	}
	sum, err := s.Summary(2)
	if err != nil || sum.AvailableCoins != 0 {
		t.Fatalf("expired still visible: %+v err=%v", sum, err)
	}
	var ledgers []Ledger
	if err := s.db.Where("biz_type = ?", LedgerExpire).Find(&ledgers).Error; err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != 1 || ledgers[0].Amount != -10 {
		t.Fatalf("expire ledger = %+v", ledgers)
	}
}

func TestRechargePackagesAndReplacePending(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	s.SetStripe(&fakeStripe{session: StripeCheckout{
		SessionID: "cs_test_pkg",
		URL:       "https://checkout.stripe.com/c/pay/cs_test_pkg",
	}})

	first, checkout, err := s.CreateRecharge(5, CreateRechargeRequest{Yuan: 30})
	if err != nil {
		t.Fatal(err)
	}
	if first.Coins != 300 || first.Bonus != 30 {
		t.Fatalf("package 30 = %+v", first)
	}
	if checkout.Method != PayMethodStripe || checkout.CheckoutURL == "" {
		t.Fatalf("checkout = %+v", checkout)
	}
	if _, _, err := s.CreateRecharge(5, CreateRechargeRequest{Yuan: 6, Method: PayMethodPage}); !errors.Is(err, ErrInvalidPayMethod) {
		t.Fatalf("page method = %v", err)
	}
	second, _, err := s.CreateRecharge(5, CreateRechargeRequest{Yuan: 6})
	if err != nil {
		t.Fatal(err)
	}
	var old RechargeOrder
	if err := s.db.Where("out_trade_no = ?", first.OutTradeNo).First(&old).Error; err != nil {
		t.Fatal(err)
	}
	if old.Status != OrderClosed || second.Status != OrderPending {
		t.Fatalf("replace pending old=%s new=%s", old.Status, second.Status)
	}
}

func TestCreateRechargeStripeAndCredit(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	fake := &fakeStripe{
		session: StripeCheckout{
			SessionID:     "cs_test_1",
			URL:           "https://checkout.stripe.com/c/pay/cs_test_1",
			PaymentStatus: "unpaid",
		},
	}
	s.SetStripe(fake)
	order, checkout, err := s.CreateRecharge(9, CreateRechargeRequest{Yuan: 6, Method: PayMethodStripe})
	if err != nil {
		t.Fatal(err)
	}
	if checkout.CheckoutURL != fake.session.URL || order.StripeSession != "cs_test_1" || order.PayMethod != PayMethodStripe {
		t.Fatalf("stripe checkout = %+v %+v", order, checkout)
	}
	fake.session.PaymentStatus = "paid"
	fake.session.PaymentRef = "pi_1"
	fake.session.AmountTotal = 600
	got, err := s.QueryOrder(9, order.OutTradeNo)
	if err != nil || got.Status != OrderPaid {
		t.Fatalf("query paid = %+v err=%v", got, err)
	}
	sum, err := s.Summary(9)
	if err != nil || sum.AvailableCoins != 60 {
		t.Fatalf("credited = %+v err=%v", sum, err)
	}
}

func TestCreateRechargeStripeNotConfigured(t *testing.T) {
	s := testService(t, time.Now())
	if _, _, err := s.CreateRecharge(1, CreateRechargeRequest{Yuan: 6, Method: PayMethodStripe}); !errors.Is(err, ErrStripeNotConfigured) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreditPaidIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	order := RechargeOrder{
		AccountID:  6,
		OutTradeNo: "WR6test",
		Yuan:       6,
		Coins:      60,
		Status:     OrderPending,
		ExpireAt:   now.Add(OrderTTL),
		CreatedAt:  now,
	}
	if err := s.db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.creditPaid("WR6test", "pi_1", "6.00"); err != nil {
		t.Fatal(err)
	}
	if err := s.creditPaid("WR6test", "pi_1", "6.00"); err != nil {
		t.Fatal(err)
	}
	sum, err := s.Summary(6)
	if err != nil || sum.AvailableCoins != 60 {
		t.Fatalf("credited twice? %+v err=%v", sum, err)
	}
	paid, err := s.FindPaidRecharge(6, "WR6test")
	if err != nil || paid == nil || paid.Yuan != 6 {
		t.Fatalf("paid lookup = %+v err=%v", paid, err)
	}
	if got, err := s.FindPaidRecharge(7, "WR6test"); err != nil || got != nil {
		t.Fatalf("other account should not see paid order: %+v err=%v", got, err)
	}
}

func TestLotterySixTiersAndPrizeIndex(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, shanghai())
	cases := []struct {
		n, index int
		coins    int64
	}{
		{0, 0, 0},
		{49, 0, 0},
		{50, 1, 2},
		{74, 1, 2},
		{75, 2, 5},
		{86, 2, 5},
		{87, 3, 10},
		{94, 3, 10},
		{95, 4, 20},
		{98, 4, 20},
		{99, 5, 50},
	}
	for i, tc := range cases {
		s := testService(t, now.Add(time.Duration(i)*24*time.Hour))
		s.draw = func() (int, error) { return tc.n, nil }
		result, err := s.Lottery(uint64(20 + i))
		if err != nil {
			t.Fatalf("n=%d: %v", tc.n, err)
		}
		if result.Coins != tc.coins || result.PrizeIndex != tc.index {
			t.Fatalf("n=%d got %+v", tc.n, result)
		}
	}
}

func TestListVideoTipsAuthorAndViewer(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s := testService(t, now)
	item := &video.Video{
		AuthorID:    30,
		Username:    "author",
		Title:       "v",
		PlayURL:     "/static/videos/a.mp4",
		AuditStatus: audit.StatusApproved,
	}
	if err := s.db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.grantInTx(s.db, 31, SourceRecharge, 100, LedgerGrantRecharge, "recharge", "seed"); err != nil {
		t.Fatal(err)
	}
	if err := s.grantInTx(s.db, 32, SourceRecharge, 100, LedgerGrantRecharge, "recharge", "seed"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Tip(31, "alice", TipRequest{VideoID: item.ID, Coins: 10}); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := s.Tip(32, "bob", TipRequest{VideoID: item.ID, Coins: 20}); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListVideoTips(30, ListVideoTipsRequest{VideoID: item.ID})
	if err != nil || len(all) != 2 {
		t.Fatalf("author list = %d err=%v", len(all), err)
	}
	own, err := s.ListVideoTips(31, ListVideoTipsRequest{VideoID: item.ID})
	if err != nil || len(own) != 1 || own[0].FromAccountID != 31 || own[0].Coins != 10 {
		t.Fatalf("viewer own = %+v err=%v", own, err)
	}
	empty, err := s.ListVideoTips(33, ListVideoTipsRequest{VideoID: item.ID})
	if err != nil || len(empty) != 0 {
		t.Fatalf("viewer empty = %+v err=%v", empty, err)
	}
}

func TestInsufficientSpend(t *testing.T) {
	s := testService(t, time.Now())
	err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.spendInTx(tx, 1, 10, LedgerConsumeTip, "tip", "x")
	})
	if !errors.Is(err, ErrInsufficient) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveRecharge(t *testing.T) {
	coins, bonus, err := resolveRecharge(68)
	if err != nil || coins != 680 || bonus != 100 {
		t.Fatalf("68 => %d+%d err=%v", coins, bonus, err)
	}
	coins, bonus, err = resolveRecharge(7)
	if err != nil || coins != 70 || bonus != 0 {
		t.Fatalf("custom 7 => %d+%d err=%v", coins, bonus, err)
	}
	if _, _, err := resolveRecharge(0); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("yuan 0: %v", err)
	}
}

type fakeStripe struct {
	session StripeCheckout
	event   StripeEvent
}

func (f *fakeStripe) CreateCheckout(StripeCheckoutRequest) (StripeCheckout, error) {
	return f.session, nil
}
func (f *fakeStripe) QuerySession(string) (StripeCheckout, error) { return f.session, nil }
func (f *fakeStripe) VerifyEvent([]byte, string, time.Time) (StripeEvent, error) {
	return f.event, nil
}
