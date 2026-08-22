package account

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	passkeyRPDisplayName = "ShortVideo"
	passkeyMaxPerAccount = 8
	passkeyNameMaxRunes  = 32
)

var (
	ErrPasskeyUnavailable = errors.New("passkey store unavailable")
	ErrPasskeyOrigin      = errors.New("passkey origin invalid")
	ErrPasskeyFailed      = errors.New("passkey verification failed")
	ErrPasskeyLimit       = errors.New("too many passkeys")
	ErrPasskeyNotFound    = errors.New("passkey not found")
	ErrPasskeyName        = errors.New("invalid passkey name")
)

// PasskeyCredential 是一条通行密钥记录。
// RecordJSON 保存 go-webauthn Credential 的完整字段，登录校验必须原样还原。
type PasskeyCredential struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	AccountID    uint64     `gorm:"index;not null" json:"account_id"`
	CredentialID []byte     `gorm:"type:varbinary(1023);not null;uniqueIndex" json:"-"`
	RecordJSON   []byte     `gorm:"type:json;not null" json:"-"`
	Name         string     `gorm:"size:64;not null" json:"name"`
	SignCount    uint32     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

func (PasskeyCredential) TableName() string {
	return "account_passkeys"
}

// PasskeyItem 是列表/登记成功后给前端的可见字段。
type PasskeyItem struct {
	ID         uint64     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type PasskeyBeginResult struct {
	SessionID string `json:"session_id"`
	Options   any    `json:"options"`
}

type PasskeyRegisterBeginRequest struct {
	Name string `json:"name"`
}

type PasskeyFinishRequest struct {
	SessionID  string          `json:"session_id" binding:"required"`
	Credential json.RawMessage `json:"credential" binding:"required"`
}

type PasskeyDeleteRequest struct {
	ID uint64 `json:"id" binding:"required"`
}

func (p PasskeyCredential) item() PasskeyItem {
	return PasskeyItem{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt}
}

type webAuthnUser struct {
	account     Account
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte {
	return accountHandle(u.account.ID)
}

func (u webAuthnUser) WebAuthnName() string {
	return u.account.Username
}

func (u webAuthnUser) WebAuthnDisplayName() string {
	return u.account.Username
}

func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func accountHandle(id uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, id)
	return buf
}

func parseAccountHandle(handle []byte) (uint64, error) {
	if len(handle) != 8 {
		return 0, ErrPasskeyFailed
	}
	return binary.BigEndian.Uint64(handle), nil
}

// newWebAuthn 按本次请求的 Origin 构造依赖方。
// 不写死域名，避免部署 overlay 吞配置；凭证会永久绑在这个 RP ID 上。
func newWebAuthn(origin string) (*webauthn.WebAuthn, error) {
	rpid, canonical, err := parsePasskeyOrigin(origin)
	if err != nil {
		return nil, err
	}
	return webauthn.New(&webauthn.Config{
		RPDisplayName: passkeyRPDisplayName,
		RPID:          rpid,
		RPOrigins:     []string{canonical},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
}

func parsePasskeyOrigin(origin string) (rpid string, canonical string, err error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", "", ErrPasskeyOrigin
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return "", "", ErrPasskeyOrigin
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", ErrPasskeyOrigin
	}
	host := u.Hostname()
	if host == "" || net.ParseIP(host) != nil {
		return "", "", ErrPasskeyOrigin
	}
	if err := protocol.ValidateRPID(host); err != nil {
		return "", "", ErrPasskeyOrigin
	}
	if u.Scheme != "https" && !isLocalhostName(host) {
		return "", "", ErrPasskeyOrigin
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", ErrPasskeyOrigin
	}
	return host, strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

func isLocalhostName(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func normalizePasskeyName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", nil
	}
	if strings.ContainsFunc(name, func(r rune) bool {
		return r < 32 || r == 127
	}) {
		return "", ErrPasskeyName
	}
	if utf8.RuneCountInString(name) > passkeyNameMaxRunes {
		return "", ErrPasskeyName
	}
	return name, nil
}

func defaultPasskeyName(cred *webauthn.Credential) string {
	kind := "通行密钥"
	if cred != nil {
		switch cred.Authenticator.Attachment {
		case protocol.Platform:
			kind = "本机"
		case protocol.CrossPlatform:
			kind = "安全密钥"
		}
	}
	return kind + " · " + time.Now().Format("01-02")
}
