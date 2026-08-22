package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

const (
	passkeySessionPrefix = "passkey:sess:"
	passkeySessionTTL    = 5 * time.Minute
)

const (
	passkeySessionRegister = "register"
	passkeySessionLogin    = "login"
)

// passkeySession 是一次仪式的服务端状态，只放 Redis，用完即删，防止挑战重放。
type passkeySession struct {
	Kind    string               `json:"kind"`
	Account uint64               `json:"account,omitempty"`
	Origin  string               `json:"origin"`
	Name    string               `json:"name,omitempty"`
	Data    webauthn.SessionData `json:"data"`
}

// PasskeySessionStore 用 Redis 保存 WebAuthn 挑战。
type PasskeySessionStore struct {
	client redis.Cmdable
	ttl    time.Duration
}

func NewPasskeySessionStore(client redis.Cmdable) *PasskeySessionStore {
	return &PasskeySessionStore{client: client, ttl: passkeySessionTTL}
}

func (s *PasskeySessionStore) available() bool {
	return s != nil && s.client != nil
}

func (s *PasskeySessionStore) Save(ctx context.Context, session passkeySession) (string, error) {
	id, err := newPasskeySessionID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("encode passkey session: %w", err)
	}
	if err := s.client.Set(ctx, passkeySessionPrefix+id, payload, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("save passkey session: %w", err)
	}
	return id, nil
}

func (s *PasskeySessionStore) Consume(ctx context.Context, id string) (*passkeySession, error) {
	if id == "" || len(id) > 64 {
		return nil, nil
	}
	key := passkeySessionPrefix + id
	value, err := s.client.GetDel(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("consume passkey session: %w", err)
	}
	var session passkeySession
	if err := json.Unmarshal(value, &session); err != nil {
		return nil, fmt.Errorf("decode passkey session: %w", err)
	}
	return &session, nil
}

func newPasskeySessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate passkey session id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
