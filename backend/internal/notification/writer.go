package notification

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Writer 在业务事实已经写入的同一事务里投影通知。
//
// 必须跟点赞 / 评论 / 关注 / 打赏的真源写在一起：通知队列另起炉灶会和
// 「点赞接口返回时关系已经可见」这条契约打架，取消点赞也可能跑到写入前面。
type Writer struct {
	repo *Repo
}

func NewWriter(db *gorm.DB) *Writer {
	return &Writer{repo: NewRepo(db)}
}

func (w *Writer) ApplyLike(tx *gorm.DB, actorID uint64, videoID uint64, authorID uint64) error {
	if w == nil || actorID == 0 || videoID == 0 || authorID == 0 || actorID == authorID {
		return nil
	}

	key := fmt.Sprintf("like:v:%d", videoID)
	now := time.Now().UTC()

	for attempt := 0; attempt < 3; attempt++ {
		row, err := w.repo.FindByDedup(tx, authorID, key)
		if err != nil {
			return err
		}
		if row == nil {
			err = w.repo.Create(tx, &Notification{
				RecipientID: authorID,
				Kind:        KindLike,
				ActorID:     actorID,
				ActorIDs:    encodeActorIDs([]uint64{actorID}),
				ActorCount:  1,
				VideoID:     videoID,
				DedupKey:    key,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			if isDuplicateKey(err) {
				continue
			}
			return err
		}

		actors := decodeActorIDs(row.ActorIDs)
		if !containsID(actors, actorID) {
			row.ActorCount++
		}
		row.ActorID = actorID
		row.ActorIDs = encodeActorIDs(prependUnique(actors, actorID, maxStoredActors))
		row.Hidden = false
		row.ReadAt = nil
		row.UpdatedAt = now
		return w.repo.Save(tx, row)
	}
	return fmt.Errorf("upsert like notification conflicted")
}

func (w *Writer) RetractLike(tx *gorm.DB, actorID uint64, videoID uint64, authorID uint64) error {
	if w == nil || actorID == 0 || videoID == 0 || authorID == 0 || actorID == authorID {
		return nil
	}

	row, err := w.repo.FindByDedup(tx, authorID, fmt.Sprintf("like:v:%d", videoID))
	if err != nil || row == nil {
		return err
	}

	actors := removeID(decodeActorIDs(row.ActorIDs), actorID)
	if row.ActorCount > 0 {
		row.ActorCount--
	}
	if row.ActorCount <= 0 || len(actors) == 0 {
		row.Hidden = true
		row.ActorCount = 0
		row.ActorIDs = "[]"
		row.ActorID = 0
	} else {
		row.ActorID = actors[0]
		row.ActorIDs = encodeActorIDs(actors)
	}
	row.UpdatedAt = time.Now().UTC()
	return w.repo.Save(tx, row)
}

func (w *Writer) ApplyFollow(tx *gorm.DB, followerID uint64, vloggerID uint64) error {
	if w == nil || followerID == 0 || vloggerID == 0 || followerID == vloggerID {
		return nil
	}

	key := fmt.Sprintf("follow:a:%d", followerID)
	now := time.Now().UTC()

	for attempt := 0; attempt < 3; attempt++ {
		row, err := w.repo.FindByDedup(tx, vloggerID, key)
		if err != nil {
			return err
		}
		if row == nil {
			err = w.repo.Create(tx, &Notification{
				RecipientID: vloggerID,
				Kind:        KindFollow,
				ActorID:     followerID,
				ActorIDs:    encodeActorIDs([]uint64{followerID}),
				ActorCount:  1,
				DedupKey:    key,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			if isDuplicateKey(err) {
				continue
			}
			return err
		}

		row.Hidden = false
		row.ReadAt = nil
		row.UpdatedAt = now
		return w.repo.Save(tx, row)
	}
	return fmt.Errorf("upsert follow notification conflicted")
}

func (w *Writer) ApplyComment(tx *gorm.DB, in CommentFanout) error {
	if w == nil || in.CommentID == 0 || in.ActorID == 0 {
		return nil
	}

	now := time.Now().UTC()
	snippet := clipText(in.Content, textClipRunes)
	rootID := in.RootCommentID
	if rootID == 0 {
		rootID = in.CommentID
	}

	already := map[uint64]struct{}{in.ActorID: {}}

	if in.ParentCommentID == 0 {
		if in.VideoAuthorID != 0 && in.VideoAuthorID != in.ActorID {
			if err := w.createOnce(tx, &Notification{
				RecipientID:   in.VideoAuthorID,
				Kind:          KindComment,
				ActorID:       in.ActorID,
				ActorIDs:      encodeActorIDs([]uint64{in.ActorID}),
				ActorCount:    1,
				VideoID:       in.VideoID,
				CommentID:     in.CommentID,
				RootCommentID: rootID,
				Text:          snippet,
				DedupKey:      fmt.Sprintf("comment:c:%d", in.CommentID),
				CreatedAt:     now,
				UpdatedAt:     now,
			}); err != nil {
				return err
			}
			already[in.VideoAuthorID] = struct{}{}
		}
	} else if in.ReplyToUserID != 0 && in.ReplyToUserID != in.ActorID {
		if err := w.createOnce(tx, &Notification{
			RecipientID:   in.ReplyToUserID,
			Kind:          KindReply,
			ActorID:       in.ActorID,
			ActorIDs:      encodeActorIDs([]uint64{in.ActorID}),
			ActorCount:    1,
			VideoID:       in.VideoID,
			CommentID:     in.CommentID,
			RootCommentID: rootID,
			Text:          snippet,
			DedupKey:      fmt.Sprintf("reply:c:%d", in.CommentID),
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
		already[in.ReplyToUserID] = struct{}{}
	}

	names := ParseMentions(in.Content)
	if len(names) == 0 {
		return nil
	}
	accounts, err := w.repo.FindAccountsByUsernamesTx(tx, names)
	if err != nil {
		return err
	}
	for _, name := range names {
		acc, ok := accounts[strings.ToLower(name)]
		if !ok {
			continue
		}
		if _, skip := already[acc.ID]; skip {
			continue
		}
		if err := w.createOnce(tx, &Notification{
			RecipientID:   acc.ID,
			Kind:          KindMention,
			ActorID:       in.ActorID,
			ActorIDs:      encodeActorIDs([]uint64{in.ActorID}),
			ActorCount:    1,
			VideoID:       in.VideoID,
			CommentID:     in.CommentID,
			RootCommentID: rootID,
			Text:          snippet,
			DedupKey:      fmt.Sprintf("mention:c:%d:u:%d", in.CommentID, acc.ID),
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
		already[acc.ID] = struct{}{}
	}
	return nil
}

func (w *Writer) HideByComment(tx *gorm.DB, commentID uint64) error {
	if w == nil {
		return nil
	}
	return w.repo.HideByComment(tx, commentID)
}

func (w *Writer) ApplyTip(tx *gorm.DB, tipID uint64, fromID uint64, toID uint64, videoID uint64, coins int64) error {
	if w == nil || tipID == 0 || fromID == 0 || toID == 0 || fromID == toID {
		return nil
	}
	now := time.Now().UTC()
	return w.createOnce(tx, &Notification{
		RecipientID: toID,
		Kind:        KindTip,
		ActorID:     fromID,
		ActorIDs:    encodeActorIDs([]uint64{fromID}),
		ActorCount:  1,
		VideoID:     videoID,
		TipID:       tipID,
		Coins:       coins,
		DedupKey:    fmt.Sprintf("tip:t:%d", tipID),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (w *Writer) createOnce(tx *gorm.DB, row *Notification) error {
	err := w.repo.Create(tx, row)
	if isDuplicateKey(err) {
		return nil
	}
	return err
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "UNIQUE constraint failed")
}
