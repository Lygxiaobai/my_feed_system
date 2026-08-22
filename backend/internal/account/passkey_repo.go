package account

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

func (r *Repo) CreatePasskey(record *PasskeyCredential) error {
	return r.db.Create(record).Error
}

func (r *Repo) ListPasskeys(accountID uint64) ([]PasskeyCredential, error) {
	var items []PasskeyCredential
	err := r.db.Where("account_id = ?", accountID).Order("id desc").Find(&items).Error
	return items, err
}

func (r *Repo) CountPasskeys(accountID uint64) (int64, error) {
	var n int64
	err := r.db.Model(&PasskeyCredential{}).Where("account_id = ?", accountID).Count(&n).Error
	return n, err
}

func (r *Repo) FindPasskeyByCredentialID(credentialID []byte) (*PasskeyCredential, error) {
	var record PasskeyCredential
	if err := r.db.Where("credential_id = ?", credentialID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *Repo) FindPasskey(accountID, id uint64) (*PasskeyCredential, error) {
	var record PasskeyCredential
	if err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *Repo) UpdatePasskeyRecord(record *PasskeyCredential) error {
	return r.db.Model(&PasskeyCredential{}).Where("id = ?", record.ID).Updates(map[string]any{
		"record_json":  record.RecordJSON,
		"sign_count":   record.SignCount,
		"last_used_at": record.LastUsedAt,
	}).Error
}

func (r *Repo) DeletePasskey(accountID, id uint64) error {
	result := r.db.Where("id = ? AND account_id = ?", id, accountID).Delete(&PasskeyCredential{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func decodePasskeyRecord(raw []byte) (webauthn.Credential, error) {
	var cred webauthn.Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return webauthn.Credential{}, err
	}
	return cred, nil
}

func encodePasskeyRecord(cred webauthn.Credential) ([]byte, error) {
	return json.Marshal(cred)
}

func credentialsOf(records []PasskeyCredential) ([]webauthn.Credential, error) {
	out := make([]webauthn.Credential, 0, len(records))
	for _, record := range records {
		cred, err := decodePasskeyRecord(record.RecordJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, cred)
	}
	return out, nil
}

func findPasskeyRecord(records []PasskeyCredential, credentialID []byte) *PasskeyCredential {
	for i := range records {
		if bytes.Equal(records[i].CredentialID, credentialID) {
			return &records[i]
		}
	}
	return nil
}

func touchPasskey(record *PasskeyCredential, cred webauthn.Credential) error {
	payload, err := encodePasskeyRecord(cred)
	if err != nil {
		return err
	}
	now := time.Now()
	record.RecordJSON = payload
	record.SignCount = cred.Authenticator.SignCount
	record.LastUsedAt = &now
	return nil
}
