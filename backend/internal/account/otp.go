package account

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	otpKeyPrefix       = "email_otp:"
	otpCooldownPrefix  = "email_otp_cd:"
	otpTestValue       = "test"
	defaultOTPTTL      = 10 * time.Minute
	defaultOTPCooldown = time.Minute
)

// OTPStore 用 Redis 保存验证码会话。测试域只记「已发送」，普通邮箱只记哈希。
type OTPStore struct {
	client   redis.Cmdable
	ttl      time.Duration
	cooldown time.Duration
}

func NewOTPStore(client redis.Cmdable, ttl time.Duration) *OTPStore {
	if ttl <= 0 {
		ttl = defaultOTPTTL
	}
	return &OTPStore{client: client, ttl: ttl, cooldown: defaultOTPCooldown}
}

func (s *OTPStore) available() bool {
	return s != nil && s.client != nil
}

func (s *OTPStore) MarkCooldown(ctx context.Context, email string) (bool, error) {
	ok, err := s.client.SetNX(ctx, otpCooldownPrefix+email, "1", s.cooldown).Result()
	if err != nil {
		return false, fmt.Errorf("set email otp cooldown: %w", err)
	}
	return ok, nil
}

func (s *OTPStore) SaveTest(ctx context.Context, email string) error {
	if err := s.client.Set(ctx, otpKeyPrefix+email, otpTestValue, s.ttl).Err(); err != nil {
		return fmt.Errorf("save test email otp: %w", err)
	}
	return nil
}

func (s *OTPStore) SaveHash(ctx context.Context, email string, hash string) error {
	if err := s.client.Set(ctx, otpKeyPrefix+email, "h:"+hash, s.ttl).Err(); err != nil {
		return fmt.Errorf("save email otp hash: %w", err)
	}
	return nil
}

func (s *OTPStore) ConsumeTest(ctx context.Context, email string) (bool, error) {
	value, err := s.peek(ctx, email)
	if err != nil || value == "" {
		return false, err
	}
	if value != otpTestValue {
		return false, nil
	}
	return true, s.delete(ctx, email)
}

func (s *OTPStore) MatchHash(ctx context.Context, email string, hash string) (bool, error) {
	value, err := s.peek(ctx, email)
	if err != nil || value == "" {
		return false, err
	}
	if len(value) < 3 || value[:2] != "h:" || value[2:] != hash {
		return false, nil
	}
	return true, s.delete(ctx, email)
}

func (s *OTPStore) peek(ctx context.Context, email string) (string, error) {
	value, err := s.client.Get(ctx, otpKeyPrefix+email).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("read email otp: %w", err)
	}
	return value, nil
}

func (s *OTPStore) delete(ctx context.Context, email string) error {
	if err := s.client.Del(ctx, otpKeyPrefix+email).Err(); err != nil {
		return fmt.Errorf("delete email otp: %w", err)
	}
	return nil
}
