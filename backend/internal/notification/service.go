package notification

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repo
}

func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepo(db)}
}

func (s *Service) List(recipientID uint64, req ListRequest) (ListResult, error) {
	if !req.Kind.ValidFilter() {
		return ListResult{}, fmt.Errorf("%w: kind", errInvalidCursor)
	}
	cursorTime, cursorID, err := decodeCursor(req.Cursor)
	if err != nil {
		return ListResult{}, errInvalidCursor
	}

	limit := req.Limit
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	rows, err := s.repo.List(recipientID, req.Kind, cursorTime, cursorID, limit+1)
	if err != nil {
		return ListResult{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items, err := s.assemble(recipientID, rows)
	if err != nil {
		return ListResult{}, err
	}

	next := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next = encodeCursor(last.UpdatedAt, last.ID)
	}
	return ListResult{Items: items, NextCursor: next, HasMore: hasMore}, nil
}

func (s *Service) UnreadCount(recipientID uint64) (UnreadCount, error) {
	return s.repo.CountUnread(recipientID)
}

func (s *Service) MarkRead(recipientID uint64, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) > maxListLimit {
		ids = ids[:maxListLimit]
	}
	_, err := s.repo.MarkRead(recipientID, ids, time.Now().UTC())
	return err
}

func (s *Service) MarkAllRead(recipientID uint64, kind Kind) error {
	if !kind.ValidFilter() {
		return errInvalidCursor
	}
	_, err := s.repo.MarkAllRead(recipientID, kind, time.Now().UTC())
	return err
}

func (s *Service) assemble(recipientID uint64, rows []Notification) ([]Item, error) {
	items := make([]Item, 0, len(rows))
	if len(rows) == 0 {
		return items, nil
	}

	accountIDs := make([]uint64, 0, len(rows)*2)
	videoIDs := make([]uint64, 0, len(rows))
	seenAccount := map[uint64]struct{}{}
	seenVideo := map[uint64]struct{}{}
	for _, row := range rows {
		for _, id := range shownActorIDs(row) {
			if _, ok := seenAccount[id]; ok || id == 0 {
				continue
			}
			seenAccount[id] = struct{}{}
			accountIDs = append(accountIDs, id)
		}
		if row.VideoID == 0 {
			continue
		}
		if _, ok := seenVideo[row.VideoID]; ok {
			continue
		}
		seenVideo[row.VideoID] = struct{}{}
		videoIDs = append(videoIDs, row.VideoID)
	}

	accounts, err := s.repo.FindAccounts(accountIDs)
	if err != nil {
		return nil, err
	}
	videos, err := s.repo.FindVideos(videoIDs)
	if err != nil {
		return nil, err
	}
	relations, err := s.repo.LoadRelations(recipientID, accountIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		actors := make([]ActorView, 0, maxShownActors)
		for _, id := range shownActorIDs(row) {
			acc, ok := accounts[id]
			if !ok {
				continue
			}
			actors = append(actors, ActorView{ID: acc.ID, Username: acc.Username})
		}
		// 账号被删时仍要能认出「有人」互动过，避免整行空白。
		if len(actors) == 0 && row.ActorID != 0 {
			actors = append(actors, ActorView{ID: row.ActorID, Username: "用户"})
		}

		primary := uint64(0)
		if len(actors) > 0 {
			primary = actors[0].ID
		}
		_, following := relations.following[primary]
		_, follower := relations.follower[primary]
		relation := ""
		if following && follower {
			relation = RelationFriend
		} else if following {
			relation = RelationFollowing
		}

		var preview *VideoPreview
		if clip, ok := videos[row.VideoID]; ok {
			preview = &VideoPreview{ID: clip.ID, CoverURL: clip.CoverURL, Title: clip.Title}
		}

		items = append(items, Item{
			ID:         row.ID,
			Kind:       row.Kind,
			Actors:     actors,
			ActorCount: row.ActorCount,
			Text:       row.Text,
			ActionText: actionText(row.Kind, row.ActorCount, row.Coins),
			Relation:   relation,
			Followed:   following,
			Video:      preview,
			Coins:      row.Coins,
			Unread:     row.ReadAt == nil,
			CreatedAt:  row.UpdatedAt,
		})
	}
	return items, nil
}

func shownActorIDs(row Notification) []uint64 {
	ids := decodeActorIDs(row.ActorIDs)
	if len(ids) == 0 && row.ActorID != 0 {
		ids = []uint64{row.ActorID}
	}
	if len(ids) > maxShownActors {
		ids = ids[:maxShownActors]
	}
	return ids
}

func actionText(kind Kind, actorCount int, coins int64) string {
	switch kind {
	case KindFollow:
		return "关注了你"
	case KindLike:
		if actorCount > 1 {
			return fmt.Sprintf("等 %d 人赞了你的作品", actorCount)
		}
		return "赞了你的作品"
	case KindComment:
		return "评论了你的作品"
	case KindReply:
		return "回复了你的评论"
	case KindMention:
		return "提到了你"
	case KindTip:
		if coins > 0 {
			return fmt.Sprintf("打赏了你的作品 %d 积分", coins)
		}
		return "打赏了你的作品"
	default:
		return ""
	}
}

var errInvalidCursor = fmt.Errorf("invalid notification cursor")
