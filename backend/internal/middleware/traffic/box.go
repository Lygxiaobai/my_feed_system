package traffic

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Box 记录拒绝次数并把反复撞限的主体送进处罚箱。
type Box interface {
	Denied(ctx context.Context, dimension string, subject string, window time.Duration) (int64, error)
	HasPenalty(ctx context.Context, dimension string, subject string) (bool, error)
	ApplyPenalty(ctx context.Context, dimension string, subject string, ttl time.Duration) error
}

type redisBox struct {
	client redis.Cmdable
	prefix string
}

func newRedisBox(client redis.Cmdable) *redisBox {
	return &redisBox{client: client, prefix: "traffic"}
}

func (b *redisBox) denyKey(dimension string, subject string) string {
	return fmt.Sprintf("%s:deny:%s:%s", b.prefix, dimension, subject)
}

func (b *redisBox) penaltyKey(dimension string, subject string) string {
	return fmt.Sprintf("%s:penalty:%s:%s", b.prefix, dimension, subject)
}

func (b *redisBox) Denied(ctx context.Context, dimension string, subject string, window time.Duration) (int64, error) {
	key := b.denyKey(dimension, subject)
	count, err := b.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := b.client.Expire(ctx, key, window).Err(); err != nil {
			_ = b.client.Del(context.Background(), key).Err()
			return 0, err
		}
	}
	return count, nil
}

func (b *redisBox) HasPenalty(ctx context.Context, dimension string, subject string) (bool, error) {
	n, err := b.client.Exists(ctx, b.penaltyKey(dimension, subject)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (b *redisBox) ApplyPenalty(ctx context.Context, dimension string, subject string, ttl time.Duration) error {
	return b.client.Set(ctx, b.penaltyKey(dimension, subject), "1", ttl).Err()
}
