package account

import (
	"context"
	"log/slog"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// SetPasskeyStore 接入通行密钥挑战存储。未设置时相关接口返回存储不可用。
func (s *Service) SetPasskeyStore(store *PasskeySessionStore) {
	s.passkeys = store
}

func (s *Service) passkeyReady() bool {
	return s.passkeys != nil && s.passkeys.available()
}

// BeginPasskeyRegister 为已登录账号发起登记仪式。
func (s *Service) BeginPasskeyRegister(ctx context.Context, origin string, accountID uint64, name string) (*PasskeyBeginResult, error) {
	name, err := normalizePasskeyName(name)
	if err != nil {
		return nil, err
	}
	if !s.passkeyReady() {
		return nil, ErrPasskeyUnavailable
	}
	n, err := s.repo.CountPasskeys(accountID)
	if err != nil {
		return nil, err
	}
	if n >= passkeyMaxPerAccount {
		return nil, ErrPasskeyLimit
	}
	wa, err := newWebAuthn(origin)
	if err != nil {
		return nil, err
	}
	user, _, err := s.loadWebAuthnUser(accountID)
	if err != nil {
		return nil, err
	}

	creation, session, err := wa.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExtensions(map[string]any{"credProps": true}),
	)
	if err != nil {
		return nil, err
	}

	sessionID, err := s.passkeys.Save(ctx, passkeySession{
		Kind:    passkeySessionRegister,
		Account: accountID,
		Origin:  origin,
		Name:    name,
		Data:    *session,
	})
	if err != nil {
		return nil, err
	}
	return &PasskeyBeginResult{SessionID: sessionID, Options: creation}, nil
}

// FinishPasskeyRegister 校验登记响应并落库。
func (s *Service) FinishPasskeyRegister(ctx context.Context, origin string, accountID uint64, req PasskeyFinishRequest) (*PasskeyItem, error) {
	if !s.passkeyReady() {
		return nil, ErrPasskeyUnavailable
	}
	wa, err := newWebAuthn(origin)
	if err != nil {
		return nil, err
	}
	session, err := s.passkeys.Consume(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.Kind != passkeySessionRegister || session.Account != accountID || session.Origin != origin {
		return nil, ErrPasskeyFailed
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(req.Credential)
	if err != nil {
		return nil, ErrPasskeyFailed
	}
	user, _, err := s.loadWebAuthnUser(accountID)
	if err != nil {
		return nil, err
	}
	cred, err := wa.CreateCredential(user, session.Data, parsed)
	if err != nil {
		slog.WarnContext(ctx, "passkey register verify failed", slog.String("error", err.Error()))
		return nil, ErrPasskeyFailed
	}

	existing, err := s.repo.FindPasskeyByCredentialID(cred.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrPasskeyFailed
	}
	n, err := s.repo.CountPasskeys(accountID)
	if err != nil {
		return nil, err
	}
	if n >= passkeyMaxPerAccount {
		return nil, ErrPasskeyLimit
	}

	payload, err := encodePasskeyRecord(*cred)
	if err != nil {
		return nil, err
	}
	name := session.Name
	if name == "" {
		name = defaultPasskeyName(cred)
	}
	record := &PasskeyCredential{
		AccountID:    accountID,
		CredentialID: cred.ID,
		RecordJSON:   payload,
		Name:         name,
		SignCount:    cred.Authenticator.SignCount,
	}
	if err := s.repo.CreatePasskey(record); err != nil {
		return nil, err
	}
	item := record.item()
	return &item, nil
}

// BeginPasskeyLogin 发起无用户名的可发现凭证登录。
func (s *Service) BeginPasskeyLogin(ctx context.Context, origin string) (*PasskeyBeginResult, error) {
	if !s.passkeyReady() {
		return nil, ErrPasskeyUnavailable
	}
	wa, err := newWebAuthn(origin)
	if err != nil {
		return nil, err
	}
	assertion, session, err := wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, err
	}
	sessionID, err := s.passkeys.Save(ctx, passkeySession{
		Kind:   passkeySessionLogin,
		Origin: origin,
		Data:   *session,
	})
	if err != nil {
		return nil, err
	}
	return &PasskeyBeginResult{SessionID: sessionID, Options: assertion}, nil
}

// FinishPasskeyLogin 校验断言并签发与其它登录方式相同的 JWT。
func (s *Service) FinishPasskeyLogin(ctx context.Context, origin string, req PasskeyFinishRequest) (*LoginResult, error) {
	if !s.passkeyReady() {
		return nil, ErrPasskeyUnavailable
	}
	wa, err := newWebAuthn(origin)
	if err != nil {
		return nil, err
	}
	session, err := s.passkeys.Consume(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.Kind != passkeySessionLogin || session.Origin != origin {
		return nil, ErrPasskeyFailed
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(req.Credential)
	if err != nil {
		return nil, ErrPasskeyFailed
	}

	user, cred, err := wa.ValidatePasskeyLogin(s.discoverPasskeyUser, session.Data, parsed)
	if err != nil {
		slog.WarnContext(ctx, "passkey login verify failed", slog.String("error", err.Error()))
		return nil, ErrPasskeyFailed
	}
	waUser, ok := user.(webAuthnUser)
	if !ok {
		return nil, ErrPasskeyFailed
	}
	if cred.Authenticator.CloneWarning {
		slog.WarnContext(ctx, "passkey clone warning", slog.Uint64("account_id", waUser.account.ID))
	}

	records, err := s.repo.ListPasskeys(waUser.account.ID)
	if err != nil {
		return nil, err
	}
	record := findPasskeyRecord(records, cred.ID)
	if record == nil {
		return nil, ErrPasskeyFailed
	}
	if err := touchPasskey(record, *cred); err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePasskeyRecord(record); err != nil {
		return nil, err
	}

	return s.persistLogin(&waUser.account)
}

// ListPasskeys 返回账号下已登记的通行密钥。
func (s *Service) ListPasskeys(accountID uint64) ([]PasskeyItem, error) {
	if _, err := s.mustAccount(accountID); err != nil {
		return nil, err
	}
	records, err := s.repo.ListPasskeys(accountID)
	if err != nil {
		return nil, err
	}
	items := make([]PasskeyItem, 0, len(records))
	for _, record := range records {
		items = append(items, record.item())
	}
	return items, nil
}

// DeletePasskey 撤销本账号的一条通行密钥。
func (s *Service) DeletePasskey(accountID, id uint64) error {
	if _, err := s.mustAccount(accountID); err != nil {
		return err
	}
	return s.repo.DeletePasskey(accountID, id)
}

func (s *Service) persistLogin(account *Account) (*LoginResult, error) {
	token, err := s.generateToken(account)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateToken(account.ID, token); err != nil {
		return nil, err
	}
	account.Token = token
	s.writeTokenCache(account.ID, token)
	return &LoginResult{Account: account, Token: token}, nil
}

func (s *Service) mustAccount(accountID uint64) (*Account, error) {
	account, err := s.repo.FindByID(accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *Service) loadWebAuthnUser(accountID uint64) (webAuthnUser, []PasskeyCredential, error) {
	account, err := s.mustAccount(accountID)
	if err != nil {
		return webAuthnUser{}, nil, err
	}
	records, err := s.repo.ListPasskeys(accountID)
	if err != nil {
		return webAuthnUser{}, nil, err
	}
	creds, err := credentialsOf(records)
	if err != nil {
		return webAuthnUser{}, nil, err
	}
	return webAuthnUser{account: *account, credentials: creds}, records, nil
}

func (s *Service) discoverPasskeyUser(rawID, userHandle []byte) (webauthn.User, error) {
	accountID, err := parseAccountHandle(userHandle)
	if err != nil {
		return nil, ErrPasskeyFailed
	}
	user, records, err := s.loadWebAuthnUser(accountID)
	if err != nil {
		return nil, err
	}
	if findPasskeyRecord(records, rawID) == nil {
		return nil, ErrPasskeyFailed
	}
	return user, nil
}
