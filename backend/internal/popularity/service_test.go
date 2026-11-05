package popularity

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestHasUsableSnapshotRequiresTenMembers(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		svc := newTestPopularityService(t)
		usable, err := svc.HasUsableSnapshot(context.Background(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if usable {
			t.Fatal("empty snapshot should not be usable")
		}
	})

	t.Run("nine", func(t *testing.T) {
		svc := newTestPopularityService(t)
		now := time.Now()
		for id := uint64(1); id <= 9; id++ {
			if err := svc.Record(context.Background(), id, 1, now); err != nil {
				t.Fatal(err)
			}
		}
		usable, err := svc.HasUsableSnapshot(context.Background(), now)
		if err != nil {
			t.Fatal(err)
		}
		if usable {
			t.Fatal("9 members should still be below the usable threshold")
		}
	})

	t.Run("ten", func(t *testing.T) {
		svc := newTestPopularityService(t)
		now := time.Now()
		for id := uint64(1); id <= 10; id++ {
			if err := svc.Record(context.Background(), id, 1, now); err != nil {
				t.Fatal(err)
			}
		}
		usable, err := svc.HasUsableSnapshot(context.Background(), now)
		if err != nil {
			t.Fatal(err)
		}
		if !usable {
			t.Fatal("10 members should make the snapshot usable")
		}
	})
}

func newTestPopularityService(t *testing.T) *Service {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return NewService(client)
}
