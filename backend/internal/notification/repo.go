package notification

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/video"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) dbOr(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *Repo) FindByDedup(tx *gorm.DB, recipientID uint64, dedupKey string) (*Notification, error) {
	var row Notification
	err := r.dbOr(tx).Where("recipient_id = ? AND dedup_key = ?", recipientID, dedupKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) Create(tx *gorm.DB, row *Notification) error {
	return r.dbOr(tx).Create(row).Error
}

func (r *Repo) Save(tx *gorm.DB, row *Notification) error {
	return r.dbOr(tx).Save(row).Error
}

func (r *Repo) HideByComment(tx *gorm.DB, commentID uint64) error {
	if commentID == 0 {
		return nil
	}
	return r.dbOr(tx).Model(&Notification{}).
		Where("comment_id = ? OR root_comment_id = ?", commentID, commentID).
		Update("hidden", true).Error
}

func (r *Repo) List(recipientID uint64, kind Kind, cursorTime time.Time, cursorID uint64, limit int) ([]Notification, error) {
	q := r.db.Where("recipient_id = ? AND hidden = ?", recipientID, false)
	q = applyKindFilter(q, kind)
	if !cursorTime.IsZero() && cursorID > 0 {
		q = q.Where("(updated_at < ?) OR (updated_at = ? AND id < ?)", cursorTime, cursorTime, cursorID)
	}
	var rows []Notification
	err := q.Order("updated_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) CountUnread(recipientID uint64) (UnreadCount, error) {
	type row struct {
		Kind  Kind
		Count int64
	}
	var rows []row
	err := r.db.Model(&Notification{}).
		Select("kind, COUNT(*) AS count").
		Where("recipient_id = ? AND hidden = ? AND read_at IS NULL", recipientID, false).
		Group("kind").
		Scan(&rows).Error
	if err != nil {
		return UnreadCount{}, err
	}

	byKind := map[Kind]int64{
		KindFollow:  0,
		KindLike:    0,
		KindMention: 0,
		KindReply:   0,
		KindTip:     0,
	}
	var total int64
	for _, item := range rows {
		total += item.Count
		switch item.Kind {
		case KindComment, KindReply:
			byKind[KindReply] += item.Count
		case KindFollow, KindLike, KindMention, KindTip:
			byKind[item.Kind] += item.Count
		}
	}
	return UnreadCount{Count: total, ByKind: byKind}, nil
}

func (r *Repo) MarkRead(recipientID uint64, ids []uint64, now time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Model(&Notification{}).
		Where("recipient_id = ? AND hidden = ? AND read_at IS NULL AND id IN ?", recipientID, false, ids).
		Update("read_at", now)
	return result.RowsAffected, result.Error
}

func (r *Repo) MarkAllRead(recipientID uint64, kind Kind, now time.Time) (int64, error) {
	q := r.db.Model(&Notification{}).
		Where("recipient_id = ? AND hidden = ? AND read_at IS NULL", recipientID, false)
	q = applyKindFilter(q, kind)
	result := q.Update("read_at", now)
	return result.RowsAffected, result.Error
}

func (r *Repo) FindAccounts(ids []uint64) (map[uint64]account.Account, error) {
	out := make(map[uint64]account.Account, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []account.Account
	if err := r.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

func (r *Repo) FindAccountsByUsernames(names []string) (map[string]account.Account, error) {
	out := make(map[string]account.Account, len(names))
	if len(names) == 0 {
		return out, nil
	}
	var rows []account.Account
	if err := r.dbOr(nil).Where("username IN ?", names).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[strings.ToLower(row.Username)] = row
	}
	return out, nil
}

func (r *Repo) FindAccountsByUsernamesTx(tx *gorm.DB, names []string) (map[string]account.Account, error) {
	out := make(map[string]account.Account, len(names))
	if len(names) == 0 {
		return out, nil
	}
	var rows []account.Account
	if err := r.dbOr(tx).Where("username IN ?", names).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[strings.ToLower(row.Username)] = row
	}
	return out, nil
}

func (r *Repo) FindVideos(ids []uint64) (map[uint64]video.Video, error) {
	out := make(map[uint64]video.Video, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []video.Video
	if err := r.db.Select("id", "cover_url", "title").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

type relationFlags struct {
	following map[uint64]struct{}
	follower  map[uint64]struct{}
}

func (r *Repo) LoadRelations(recipientID uint64, actorIDs []uint64) (relationFlags, error) {
	flags := relationFlags{
		following: map[uint64]struct{}{},
		follower:  map[uint64]struct{}{},
	}
	if len(actorIDs) == 0 {
		return flags, nil
	}

	var followingIDs []uint64
	if err := r.db.Table("social_relations").
		Where("follower_id = ? AND vlogger_id IN ?", recipientID, actorIDs).
		Pluck("vlogger_id", &followingIDs).Error; err != nil {
		return flags, err
	}
	for _, id := range followingIDs {
		flags.following[id] = struct{}{}
	}

	var followerIDs []uint64
	if err := r.db.Table("social_relations").
		Where("vlogger_id = ? AND follower_id IN ?", recipientID, actorIDs).
		Pluck("follower_id", &followerIDs).Error; err != nil {
		return flags, err
	}
	for _, id := range followerIDs {
		flags.follower[id] = struct{}{}
	}
	return flags, nil
}

func applyKindFilter(q *gorm.DB, kind Kind) *gorm.DB {
	switch kind {
	case KindFollow, KindLike, KindMention, KindTip:
		return q.Where("kind = ?", kind)
	case KindReply, KindComment:
		return q.Where("kind IN ?", []Kind{KindReply, KindComment})
	default:
		return q
	}
}

func encodeCursor(t time.Time, id uint64) string {
	if t.IsZero() || id == 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", t.UnixMilli(), id)
}

func decodeCursor(raw string) (time.Time, uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, 0, nil
	}
	left, right, ok := strings.Cut(raw, ":")
	if !ok {
		return time.Time{}, 0, fmt.Errorf("invalid cursor")
	}
	ms, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor")
	}
	id, err := strconv.ParseUint(right, 10, 64)
	if err != nil || id == 0 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor")
	}
	return time.UnixMilli(ms).UTC(), id, nil
}

func encodeActorIDs(ids []uint64) string {
	if len(ids) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func decodeActorIDs(raw string) []uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []uint64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

func prependUnique(ids []uint64, id uint64, max int) []uint64 {
	out := make([]uint64, 0, len(ids)+1)
	out = append(out, id)
	for _, existing := range ids {
		if existing == id {
			continue
		}
		out = append(out, existing)
		if len(out) >= max {
			break
		}
	}
	return out
}

func removeID(ids []uint64, id uint64) []uint64 {
	out := ids[:0]
	for _, existing := range ids {
		if existing == id {
			continue
		}
		out = append(out, existing)
	}
	return out
}

func containsID(ids []uint64, id uint64) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}
