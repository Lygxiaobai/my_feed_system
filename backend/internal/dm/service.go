package dm

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrEmptyBody     = errors.New("dm body is empty")
	ErrBodyTooLong   = errors.New("dm body is too long")
	ErrSelf          = errors.New("cannot message self")
	ErrPeerMissing   = errors.New("peer account missing")
	ErrQuotaExceeded = errors.New("stranger dm quota exceeded")
	ErrPeerRequired  = errors.New("peer id required")
)

type Service struct {
	repo *Repo
	db   *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepo(db), db: db}
}

func (s *Service) Inbox(me uint64) (InboxResult, error) {
	rows, err := s.repo.ListInbox(me, maxInboxSize)
	if err != nil {
		return InboxResult{}, err
	}
	peerIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		peerIDs = append(peerIDs, row.PeerID(me))
	}
	accounts, err := s.repo.FindAccounts(peerIDs)
	if err != nil {
		return InboxResult{}, err
	}

	items := make([]ConversationView, 0, len(rows))
	for _, row := range rows {
		peerID := row.PeerID(me)
		name := "用户"
		if acc, ok := accounts[peerID]; ok {
			name = acc.Username
		}
		items = append(items, ConversationView{
			Peer:         PeerView{ID: peerID, Username: name},
			Preview:      row.LastPreview,
			Unread:       row.UnreadOf(me),
			LastAt:       row.LastAt,
			LastSenderID: row.LastSenderID,
		})
	}
	return InboxResult{Items: items}, nil
}

func (s *Service) UnreadCount(me uint64) (UnreadCount, error) {
	n, err := s.repo.SumUnread(me)
	if err != nil {
		return UnreadCount{}, err
	}
	return UnreadCount{Count: n}, nil
}

func (s *Service) Thread(me uint64, req ThreadRequest) (ThreadResult, error) {
	if req.PeerID == 0 {
		return ThreadResult{}, ErrPeerRequired
	}
	if req.PeerID == me {
		return ThreadResult{}, ErrSelf
	}

	acc, err := s.repo.FindAccount(req.PeerID)
	if err != nil {
		return ThreadResult{}, err
	}
	conv, err := s.repo.FindConversation(me, req.PeerID)
	if err != nil {
		return ThreadResult{}, err
	}
	if acc == nil && conv == nil {
		return ThreadResult{}, ErrPeerMissing
	}

	peer := PeerView{ID: req.PeerID, Username: "用户"}
	if acc != nil {
		peer.Username = acc.Username
	}

	following, follower, err := s.repo.RelationFlags(me, req.PeerID)
	if err != nil {
		return ThreadResult{}, err
	}
	relation := relationOf(following, follower)

	limit := req.Limit
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	var messages []Message
	var hasMore bool
	if conv != nil {
		rows, listErr := s.repo.ListMessages(conv.ID, req.AfterID, req.BeforeID, limit+1)
		if listErr != nil {
			return ThreadResult{}, listErr
		}
		hasMore = len(rows) > limit
		if hasMore {
			// after_id 是正序多取了一条最新的；首屏和 before_id 在反转后多的是最旧的。
			if req.AfterID > 0 {
				rows = rows[:limit]
			} else {
				rows = rows[1:]
			}
		}
		messages = rows
		// 打开或追新消息时标已读；翻历史不必再写一次，但标了也不伤。
		if req.BeforeID == 0 {
			if err := s.repo.MarkRead(conv, me, time.Now().UTC()); err != nil {
				return ThreadResult{}, err
			}
			fresh, freshErr := s.repo.FindConversation(me, req.PeerID)
			if freshErr != nil {
				return ThreadResult{}, freshErr
			}
			if fresh != nil {
				conv = fresh
			}
		}
	}

	sent := 0
	if conv != nil {
		n, countErr := s.repo.CountSent(s.db, conv.ID, me)
		if countErr != nil {
			return ThreadResult{}, countErr
		}
		sent = int(n)
	}

	canSend, remaining := quotaState(relation == RelationFriend, sent)
	if acc == nil {
		canSend = false
		remaining = 0
	}

	var peerRead *time.Time
	if conv != nil {
		peerRead = conv.LastReadAt(req.PeerID)
	}

	return ThreadResult{
		Peer:      peer,
		Relation:  relation,
		CanSend:   canSend,
		Remaining: remaining,
		Messages:  toMessageViews(me, messages, peerRead),
		HasMore:   hasMore,
	}, nil
}

func (s *Service) MarkRead(me, peerID uint64) error {
	if peerID == 0 {
		return ErrPeerRequired
	}
	if peerID == me {
		return ErrSelf
	}
	conv, err := s.repo.FindConversation(me, peerID)
	if err != nil || conv == nil {
		return err
	}
	return s.repo.MarkRead(conv, me, time.Now().UTC())
}

func (s *Service) Send(me uint64, req SendRequest) (SendResult, error) {
	if req.PeerID == 0 {
		return SendResult{}, ErrPeerRequired
	}
	if req.PeerID == me {
		return SendResult{}, ErrSelf
	}
	body, err := normalizeBody(req.Content)
	if err != nil {
		return SendResult{}, err
	}

	acc, err := s.repo.FindAccount(req.PeerID)
	if err != nil {
		return SendResult{}, err
	}
	if acc == nil {
		return SendResult{}, ErrPeerMissing
	}

	following, follower, err := s.repo.RelationFlags(me, req.PeerID)
	if err != nil {
		return SendResult{}, err
	}
	relation := relationOf(following, follower)
	mutual := relation == RelationFriend

	var result SendResult
	now := time.Now().UTC()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		conv, ensErr := s.repo.EnsureConversation(tx, me, req.PeerID, now)
		if ensErr != nil {
			return ensErr
		}
		sent, countErr := s.repo.CountSent(tx, conv.ID, me)
		if countErr != nil {
			return countErr
		}
		if !mutual && sent >= StrangerQuota {
			return ErrQuotaExceeded
		}

		msg := &Message{
			ConversationID: conv.ID,
			SenderID:       me,
			Body:           body,
			CreatedAt:      now,
		}
		if insErr := s.repo.InsertMessage(tx, msg); insErr != nil {
			return insErr
		}
		if touchErr := s.repo.TouchAfterSend(tx, conv, msg, me, clipPreview(body), now); touchErr != nil {
			return touchErr
		}

		canSend, remaining := quotaState(mutual, int(sent)+1)
		result = SendResult{
			Message: MessageView{
				ID:        msg.ID,
				SenderID:  me,
				Body:      body,
				Mine:      true,
				Read:      false,
				CreatedAt: msg.CreatedAt,
			},
			Relation:  relation,
			CanSend:   canSend,
			Remaining: remaining,
		}
		return nil
	})
	if err != nil {
		return SendResult{}, err
	}
	return result, nil
}

func normalizeBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", ErrEmptyBody
	}
	if utf8.RuneCountInString(body) > MaxBodyRunes {
		return "", ErrBodyTooLong
	}
	return body, nil
}

func clipPreview(body string) string {
	collapsed := strings.Join(strings.Fields(strings.ReplaceAll(body, "\n", " ")), " ")
	if utf8.RuneCountInString(collapsed) <= PreviewRunes {
		return collapsed
	}
	runes := []rune(collapsed)
	return string(runes[:PreviewRunes]) + "…"
}

func relationOf(following, follower bool) string {
	switch {
	case following && follower:
		return RelationFriend
	case following:
		return RelationFollowing
	case follower:
		return RelationFollower
	default:
		return RelationNone
	}
}

func quotaState(mutual bool, sent int) (bool, int) {
	if mutual {
		return true, -1
	}
	left := StrangerQuota - sent
	if left < 0 {
		left = 0
	}
	return left > 0, left
}

func toMessageViews(me uint64, rows []Message, peerRead *time.Time) []MessageView {
	out := make([]MessageView, 0, len(rows))
	for _, row := range rows {
		read := false
		if row.SenderID == me && peerRead != nil && !row.CreatedAt.After(*peerRead) {
			read = true
		}
		out = append(out, MessageView{
			ID:        row.ID,
			SenderID:  row.SenderID,
			Body:      row.Body,
			Mine:      row.SenderID == me,
			Read:      read,
			CreatedAt: row.CreatedAt,
		})
	}
	return out
}
