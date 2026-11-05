package recommend

import (
	"testing"
	"time"

	"my_feed_system/internal/video"
)

func TestSlotPlanTenHasTwoSmall(t *testing.T) {
	plan := slotPlan(10, 0.2)
	if len(plan) != 10 {
		t.Fatalf("len=%d", len(plan))
	}
	var small, hot int
	for _, k := range plan {
		if k == queueSmall {
			small++
		}
		if k == queueHot {
			hot++
		}
	}
	if small != 2 {
		t.Fatalf("small slots=%d want 2 plan=%v", small, plan)
	}
	if hot != 2 {
		t.Fatalf("hot slots=%d want 2", hot)
	}
}

func TestMixSmallCreatorQuota(t *testing.T) {
	cfg := defaultMixConfig()
	cfg.SmallMaxFollowers = 10
	now := time.Now()
	cands := make([]candidate, 0, 12)
	for i := 1; i <= 6; i++ {
		cands = append(cands, candidate{
			Video:         video.Video{ID: uint64(i), AuthorID: 100, Title: "big", CreatedAt: now.Add(-time.Duration(i) * time.Hour)},
			FollowerCount: 80,
			Hot:           int64(100 - i),
			Rel:           0.9,
			Vector:        []float32{1, 0},
		})
	}
	for i := 7; i <= 12; i++ {
		cands = append(cands, candidate{
			Video:         video.Video{ID: uint64(i), AuthorID: uint64(200 + i), Title: "small", CreatedAt: now.Add(-time.Duration(i) * time.Minute)},
			FollowerCount: 1,
			Hot:           1,
			Rel:           0.4,
			Vector:        []float32{0, 1},
		})
	}
	user := []float32{1, 0}
	interest, small, hot := splitQueues(cands, user, cfg)
	picked := mix(interest, small, hot, 10, cfg)
	if len(picked) != 10 {
		t.Fatalf("picked=%d", len(picked))
	}
	var smallCount int
	for _, item := range picked {
		if item.FollowerCount < cfg.SmallMaxFollowers {
			smallCount++
		}
	}
	if smallCount < 2 {
		t.Fatalf("small creators in page=%d want >=2", smallCount)
	}
}

func TestMixAuthorWindow(t *testing.T) {
	cfg := defaultMixConfig()
	cfg.SmallMaxFollowers = 1
	now := time.Now()
	cands := []candidate{}
	oneHot := func(axis int) []float32 {
		v := make([]float32, 16)
		v[axis] = 1
		return v
	}
	for i := 1; i <= 8; i++ {
		cands = append(cands, candidate{
			Video:         video.Video{ID: uint64(i), AuthorID: 1, CreatedAt: now.Add(-time.Duration(i) * time.Minute)},
			FollowerCount: 99,
			Hot:           int64(20 - i),
			Vector:        oneHot(0),
			Rel:           0.8,
		})
	}
	for i := 9; i <= 16; i++ {
		cands = append(cands, candidate{
			Video:         video.Video{ID: uint64(i), AuthorID: uint64(i), CreatedAt: now.Add(-time.Duration(i) * time.Minute)},
			FollowerCount: 99,
			Hot:           3,
			Vector:        oneHot(i - 1),
			Rel:           0.5,
		})
	}
	interest, small, hot := splitQueues(cands, oneHot(0), cfg)
	picked := mix(interest, small, hot, 10, cfg)
	for i := 1; i < len(picked); i++ {
		windowStart := i - cfg.AuthorWindow
		if windowStart < 0 {
			windowStart = 0
		}
		for j := windowStart; j < i; j++ {
			if picked[i].Video.AuthorID == picked[j].Video.AuthorID {
				t.Fatalf("author %d repeats within window at %d and %d", picked[i].Video.AuthorID, j, i)
			}
		}
	}
}

func TestMixEmptySmallFillsFromInterest(t *testing.T) {
	cfg := defaultMixConfig()
	cfg.SmallMaxFollowers = 1
	now := time.Now()
	cands := make([]candidate, 0, 8)
	for i := 1; i <= 8; i++ {
		cands = append(cands, candidate{
			Video:         video.Video{ID: uint64(i), AuthorID: uint64(i), CreatedAt: now},
			FollowerCount: 80,
			Hot:           int64(i),
			Vector:        []float32{1, 0},
		})
	}
	interest, small, hot := splitQueues(cands, []float32{1, 0}, cfg)
	if len(small) != 0 {
		t.Fatalf("small should be empty, got %d", len(small))
	}
	picked := mix(interest, small, hot, 10, cfg)
	if len(picked) < 8 {
		t.Fatalf("expected interest backfill, got %d", len(picked))
	}
}
