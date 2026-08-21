package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/feed"
	"my_feed_system/internal/mq"
	"my_feed_system/internal/social"
)

type SocialWorker struct {
	db        *gorm.DB
	inbox     *feed.InboxStore
	following *feed.FollowingCache
}

func NewSocialWorker(db *gorm.DB) *SocialWorker {
	return NewSocialWorkerWithFanout(db, nil, nil)
}

// NewSocialWorkerWithFanout 额外接入关注流缓存，使关注关系变化能立刻反映到关注流。
func NewSocialWorkerWithFanout(db *gorm.DB, inbox *feed.InboxStore, following *feed.FollowingCache) *SocialWorker {
	return &SocialWorker{
		db:        db,
		inbox:     inbox,
		following: following,
	}
}

func (w *SocialWorker) Handle(ctx context.Context, event mq.Envelope) error {
	switch event.EventType {
	case mq.EventTypeSocialFollowed:
		return w.handleFollowed(ctx, event)
	case mq.EventTypeSocialUnfollowed:
		return w.handleUnfollowed(ctx, event)
	default:
		return fmt.Errorf("social worker unsupported event: %s", event.EventType)
	}
}

func (w *SocialWorker) handleFollowed(ctx context.Context, event mq.Envelope) error {
	var payload mq.SocialPayload
	if err := event.DecodePayload(&payload); err != nil {
		return fmt.Errorf("decode social.followed payload: %w", err)
	}
	if payload.FollowerID == 0 || payload.VloggerID == 0 {
		return errors.New("invalid social.followed payload")
	}

	err := w.db.Transaction(func(tx *gorm.DB) error {
		// Step 1: idempotency gate.
		if err := mq.MarkProcessed(tx, "social-worker", event); err != nil {
			if errors.Is(err, mq.ErrAlreadyProcessed) {
				return nil
			}
			return err
		}

		createErr := tx.Create(&social.SocialRelation{
			FollowerID: payload.FollowerID,
			VloggerID:  payload.VloggerID,
		}).Error
		if createErr != nil {
			if !isDuplicateKey(createErr) {
				return createErr
			}
			// 关系已存在，粉丝数不能再加一次。
			return nil
		}

		// 粉丝数与关注关系同事务增减，避免计数与真值分叉。
		return tx.Model(&account.Account{}).
			Where("id = ?", payload.VloggerID).
			UpdateColumn("follower_count", gorm.Expr("follower_count + 1")).Error
	})
	if err != nil {
		return err
	}

	w.resetFollowingFeed(ctx, payload.FollowerID)
	return nil
}

func (w *SocialWorker) handleUnfollowed(ctx context.Context, event mq.Envelope) error {
	var payload mq.SocialPayload
	if err := event.DecodePayload(&payload); err != nil {
		return fmt.Errorf("decode social.unfollowed payload: %w", err)
	}
	if payload.FollowerID == 0 || payload.VloggerID == 0 {
		return errors.New("invalid social.unfollowed payload")
	}

	err := w.db.Transaction(func(tx *gorm.DB) error {
		// Step 1: idempotency gate.
		if err := mq.MarkProcessed(tx, "social-worker", event); err != nil {
			if errors.Is(err, mq.ErrAlreadyProcessed) {
				return nil
			}
			return err
		}

		result := tx.Where("follower_id = ? AND vlogger_id = ?", payload.FollowerID, payload.VloggerID).
			Delete(&social.SocialRelation{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		// 计数下限由 WHERE 保护：异步链路里任何一次漏加都会让减法把计数带成负数，
		// 而负数会让该作者被永久判成普通用户。
		return tx.Model(&account.Account{}).
			Where("id = ? AND follower_count > 0", payload.VloggerID).
			UpdateColumn("follower_count", gorm.Expr("follower_count - 1")).Error
	})
	if err != nil {
		return err
	}

	w.resetFollowingFeed(ctx, payload.FollowerID)
	return nil
}

// resetFollowingFeed 让该用户的关注流按新的关注关系重新组织。
//
// 清掉关注列表缓存是为了让读路径立刻看到新关系；清掉活跃标记则会触发一次
// 回源重建，把新关注作者的历史视频补进收件箱。相比在这里精确增删收件箱成员，
// 重建一次的代价只是下次读关注流多一条查询，却省掉了一整套补偿逻辑。
func (w *SocialWorker) resetFollowingFeed(ctx context.Context, followerID uint64) {
	if err := w.following.Delete(ctx, followerID); err != nil {
		slog.WarnContext(ctx, "invalidate following cache failed",
			slog.Uint64("follower_id", followerID), slog.String("error", err.Error()))
	}
	if err := w.inbox.ClearActive(ctx, followerID); err != nil {
		slog.WarnContext(ctx, "clear following inbox active flag failed",
			slog.Uint64("follower_id", followerID), slog.String("error", err.Error()))
	}
}
