package dm

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"my_feed_system/internal/account"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func pair(a, b uint64) (uint64, uint64) {
	if a < b {
		return a, b
	}
	return b, a
}

func (r *Repo) FindAccount(id uint64) (*account.Account, error) {
	var row account.Account
	err := r.db.Select("id", "username").Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) FindAccounts(ids []uint64) (map[uint64]account.Account, error) {
	out := make(map[uint64]account.Account, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []account.Account
	if err := r.db.Select("id", "username").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

// RelationFlags 返回「我是否关注对方」以及「对方是否关注我」。
func (r *Repo) RelationFlags(me, peer uint64) (following, follower bool, err error) {
	var pairs []struct {
		FollowerID uint64
		VloggerID  uint64
	}
	err = r.db.Table("social_relations").
		Select("follower_id", "vlogger_id").
		Where("(follower_id = ? AND vlogger_id = ?) OR (follower_id = ? AND vlogger_id = ?)", me, peer, peer, me).
		Find(&pairs).Error
	if err != nil {
		return false, false, err
	}
	for _, row := range pairs {
		if row.FollowerID == me && row.VloggerID == peer {
			following = true
		}
		if row.FollowerID == peer && row.VloggerID == me {
			follower = true
		}
	}
	return following, follower, nil
}

func (r *Repo) FindConversation(me, peer uint64) (*Conversation, error) {
	lo, hi := pair(me, peer)
	var row Conversation
	err := r.db.Where("user_lo = ? AND user_hi = ?", lo, hi).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// EnsureConversation 在同一事务里拿到会话行并锁定，避免陌生人并发连发绕过额度。
func (r *Repo) EnsureConversation(tx *gorm.DB, me, peer uint64, now time.Time) (*Conversation, error) {
	lo, hi := pair(me, peer)
	seed := Conversation{UserLo: lo, UserHi: hi, LastAt: now, CreatedAt: now}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_lo"}, {Name: "user_hi"}},
		DoNothing: true,
	}).Create(&seed).Error; err != nil {
		return nil, err
	}

	var row Conversation
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_lo = ? AND user_hi = ?", lo, hi).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) CountSent(tx *gorm.DB, conversationID, senderID uint64) (int64, error) {
	var n int64
	err := tx.Model(&Message{}).
		Where("conversation_id = ? AND sender_id = ?", conversationID, senderID).
		Count(&n).Error
	return n, err
}

func (r *Repo) InsertMessage(tx *gorm.DB, row *Message) error {
	return tx.Create(row).Error
}

func (r *Repo) TouchAfterSend(tx *gorm.DB, conv *Conversation, msg *Message, me uint64, preview string, now time.Time) error {
	updates := map[string]any{
		"last_message_id": msg.ID,
		"last_preview":    preview,
		"last_sender_id":  me,
		"last_at":         now,
	}
	if me == conv.UserLo {
		updates["hi_unread"] = conv.HiUnread + 1
		updates["lo_last_read_at"] = now
	} else {
		updates["lo_unread"] = conv.LoUnread + 1
		updates["hi_last_read_at"] = now
	}
	return tx.Model(&Conversation{}).Where("id = ?", conv.ID).Updates(updates).Error
}

func (r *Repo) MarkRead(conv *Conversation, me uint64, now time.Time) error {
	updates := map[string]any{}
	if me == conv.UserLo {
		updates["lo_unread"] = 0
		updates["lo_last_read_at"] = now
	} else {
		updates["hi_unread"] = 0
		updates["hi_last_read_at"] = now
	}
	return r.db.Model(&Conversation{}).Where("id = ?", conv.ID).Updates(updates).Error
}

func (r *Repo) ListInbox(me uint64, limit int) ([]Conversation, error) {
	var rows []Conversation
	err := r.db.Where("user_lo = ? OR user_hi = ?", me, me).
		Order("last_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repo) SumUnread(me uint64) (int64, error) {
	var loSum, hiSum int64
	if err := r.db.Model(&Conversation{}).
		Where("user_lo = ?", me).
		Select("COALESCE(SUM(lo_unread),0)").
		Scan(&loSum).Error; err != nil {
		return 0, err
	}
	if err := r.db.Model(&Conversation{}).
		Where("user_hi = ?", me).
		Select("COALESCE(SUM(hi_unread),0)").
		Scan(&hiSum).Error; err != nil {
		return 0, err
	}
	return loSum + hiSum, nil
}

func (r *Repo) ListMessages(conversationID, afterID, beforeID uint64, limit int) ([]Message, error) {
	q := r.db.Where("conversation_id = ?", conversationID)
	var rows []Message
	switch {
	case afterID > 0:
		err := q.Where("id > ?", afterID).Order("id ASC").Limit(limit).Find(&rows).Error
		return rows, err
	case beforeID > 0:
		err := q.Where("id < ?", beforeID).Order("id DESC").Limit(limit).Find(&rows).Error
		if err != nil {
			return nil, err
		}
		reverseMessages(rows)
		return rows, nil
	default:
		err := q.Order("id DESC").Limit(limit).Find(&rows).Error
		if err != nil {
			return nil, err
		}
		reverseMessages(rows)
		return rows, nil
	}
}

func reverseMessages(rows []Message) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}
