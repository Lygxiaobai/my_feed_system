package wallet

import (
	"errors"
	"testing"
)

func TestDrawCheckinPrizeRangeAndOddsBuckets(t *testing.T) {
	if got := drawCheckinPrize(0); got != 1 {
		t.Fatalf("0 => %d", got)
	}
	if got := drawCheckinPrize(94); got != 1 {
		t.Fatalf("94 => %d", got)
	}
	if got := drawCheckinPrize(95); got != 2 {
		t.Fatalf("95 => %d", got)
	}
	if got := drawCheckinPrize(949); got != 10 {
		t.Fatalf("949 => %d", got)
	}
	if got := drawCheckinPrize(950); got != 11 {
		t.Fatalf("950 => %d", got)
	}
	if got := drawCheckinPrize(999); got != 20 {
		t.Fatalf("999 => %d", got)
	}
	if got := drawCheckinPrize(1000); got != 1 {
		t.Fatalf("1000 wraps => %d", got)
	}

	var low, high int
	for n := 0; n < CheckinBucketMod; n++ {
		prize := drawCheckinPrize(n)
		if prize < 1 || prize > 20 {
			t.Fatalf("n=%d prize=%d out of range", n, prize)
		}
		if prize <= 10 {
			low++
		} else {
			high++
		}
	}
	if low != 950 || high != 50 {
		t.Fatalf("low=%d high=%d", low, high)
	}
}

func TestNormalizePayMethod(t *testing.T) {
	got, err := normalizePayMethod("")
	if err != nil || got != PayMethodStripe {
		t.Fatalf("empty => %s %v", got, err)
	}
	if _, err := normalizePayMethod("page"); !errors.Is(err, ErrInvalidPayMethod) {
		t.Fatalf("page => %v", err)
	}
	got, err = normalizePayMethod("stripe")
	if err != nil || got != PayMethodStripe {
		t.Fatalf("stripe => %s %v", got, err)
	}
	if _, err := normalizePayMethod("wechat"); err == nil {
		t.Fatal("unknown method must fail")
	}
}

func TestParseYuanAmount(t *testing.T) {
	n, err := parseYuanAmount("6.00")
	if err != nil || n != 6 {
		t.Fatalf("6.00 => %d %v", n, err)
	}
	if _, err := parseYuanAmount("6.01"); err == nil {
		t.Fatal("fractional yuan should fail")
	}
}

func TestDrawLotterySixTiers(t *testing.T) {
	counts := make([]int, 6)
	for n := 0; n < LotteryBucketMod; n++ {
		coins, idx := drawLottery(n)
		if idx < 0 || idx >= len(LotteryPrizes) {
			t.Fatalf("n=%d idx=%d", n, idx)
		}
		if coins != LotteryPrizes[idx] {
			t.Fatalf("n=%d coins=%d idx=%d", n, coins, idx)
		}
		counts[idx]++
	}
	want := []int{50, 25, 12, 8, 4, 1}
	for i, w := range want {
		if counts[i] != w {
			t.Fatalf("tier %d count=%d want=%d", i, counts[i], w)
		}
	}
}
