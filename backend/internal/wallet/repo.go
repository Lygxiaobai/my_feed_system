package wallet

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) availableLots(tx *gorm.DB, accountID uint64, now time.Time) ([]Lot, error) {
	var lots []Lot
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ? AND remaining > 0 AND (expire_at IS NULL OR expire_at > ?)", accountID, now).
		Order("CASE WHEN expire_at IS NULL THEN 1 ELSE 0 END ASC, expire_at ASC, created_at ASC, id ASC").
		Find(&lots).Error
	return lots, err
}

func (r *Repo) summary(tx *gorm.DB, accountID uint64, now time.Time) (Summary, error) {
	type row struct {
		Remaining int64
		ExpireAt  *time.Time
	}
	var rows []row
	err := tx.Model(&Lot{}).
		Select("remaining, expire_at").
		Where("account_id = ? AND remaining > 0 AND (expire_at IS NULL OR expire_at > ?)", accountID, now).
		Find(&rows).Error
	if err != nil {
		return Summary{}, err
	}

	var out Summary
	warnUntil := now.Add(ExpireWarnWindow)
	var nextAt *time.Time
	var nextCoins int64
	for _, item := range rows {
		out.AvailableCoins += item.Remaining
		if item.ExpireAt == nil {
			continue
		}
		if !item.ExpireAt.After(warnUntil) {
			out.ExpiringSoonCoins += item.Remaining
		}
		if nextAt == nil || item.ExpireAt.Before(*nextAt) {
			t := *item.ExpireAt
			nextAt = &t
			nextCoins = item.Remaining
		} else if item.ExpireAt.Equal(*nextAt) {
			nextCoins += item.Remaining
		}
	}
	out.NextExpireAt = nextAt
	out.NextExpireCoins = nextCoins
	return out, nil
}

func (r *Repo) hasRegisterGift(tx *gorm.DB, accountID uint64) (bool, error) {
	var n int64
	err := tx.Model(&Lot{}).
		Where("account_id = ? AND source = ?", accountID, SourceRegister).
		Limit(1).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	err = tx.Model(&Ledger{}).
		Where("account_id = ? AND biz_type = ?", accountID, LedgerGrantRegister).
		Limit(1).
		Count(&n).Error
	return n > 0, err
}

func (r *Repo) expiredLots(tx *gorm.DB, now time.Time, limit int) ([]Lot, error) {
	var lots []Lot
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("remaining > 0 AND expire_at IS NOT NULL AND expire_at <= ?", now).
		Order("id ASC").
		Limit(limit).
		Find(&lots).Error
	return lots, err
}

func (r *Repo) expiredLotsForAccount(tx *gorm.DB, accountID uint64, now time.Time) ([]Lot, error) {
	var lots []Lot
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ? AND remaining > 0 AND expire_at IS NOT NULL AND expire_at <= ?", accountID, now).
		Order("id ASC").
		Find(&lots).Error
	return lots, err
}

func (r *Repo) checkinDaysInRange(tx *gorm.DB, accountID uint64, start, end string) ([]DailyAction, error) {
	var items []DailyAction
	err := tx.Where("account_id = ? AND action = ? AND biz_date >= ? AND biz_date <= ?",
		accountID, ActionCheckin, start, end).
		Order("biz_date ASC").
		Find(&items).Error
	return items, err
}

func (r *Repo) lastTip(tx *gorm.DB, accountID, videoID uint64) (*TipRecord, error) {
	var rec TipRecord
	err := tx.Where("from_account_id = ? AND video_id = ?", accountID, videoID).
		Order("id DESC").
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repo) findOrderByOutTradeNo(tx *gorm.DB, outTradeNo string) (*RechargeOrder, error) {
	var order RechargeOrder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("out_trade_no = ?", outTradeNo).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repo) pendingOrders(tx *gorm.DB, accountID uint64) ([]RechargeOrder, error) {
	var orders []RechargeOrder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ? AND status = ?", accountID, OrderPending).
		Find(&orders).Error
	return orders, err
}

func (r *Repo) duePendingOrders(tx *gorm.DB, now time.Time, limit int) ([]RechargeOrder, error) {
	var orders []RechargeOrder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ? AND expire_at <= ?", OrderPending, now).
		Order("id ASC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}
