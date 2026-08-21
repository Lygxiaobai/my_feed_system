package recommend

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const interestCacheKeyPrefix = "recommend:interest:"

// InterestCache 缓存用户兴趣向量。点赞/打赏后失效，主数据仍是 likes/tips + video_embeddings。
type InterestCache struct {
	client redis.Cmdable
}

func NewInterestCache(client redis.Cmdable) *InterestCache {
	if client == nil {
		return nil
	}
	return &InterestCache{client: client}
}

func (c *InterestCache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *InterestCache) InvalidateUser(accountID uint64) {
	if !c.Enabled() || accountID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.client.Del(ctx, interestKey(accountID)).Err(); err != nil {
		slog.Warn("invalidate interest cache failed",
			slog.Uint64("account_id", accountID), slog.String("error", err.Error()))
	}
}

func (c *InterestCache) get(ctx context.Context, accountID uint64, model string) ([]float32, bool) {
	if !c.Enabled() || accountID == 0 {
		return nil, false
	}
	raw, err := c.client.Get(ctx, interestKey(accountID)).Bytes()
	if err != nil {
		return nil, false
	}
	vec, gotModel, ok := decodeInterestCache(raw)
	if !ok || gotModel != model {
		return nil, false
	}
	return vec, true
}

func (c *InterestCache) set(ctx context.Context, accountID uint64, model string, vec []float32) {
	if !c.Enabled() || accountID == 0 || len(vec) == 0 {
		return
	}
	if err := c.client.Set(ctx, interestKey(accountID), encodeInterestCache(model, vec), interestCacheTTL).Err(); err != nil {
		slog.Warn("write interest cache failed",
			slog.Uint64("account_id", accountID), slog.String("error", err.Error()))
	}
}

func interestKey(accountID uint64) string {
	return interestCacheKeyPrefix + strconv.FormatUint(accountID, 10)
}

func encodeInterestCache(model string, vec []float32) []byte {
	modelBytes := []byte(model)
	buf := make([]byte, 4+len(modelBytes)+4+4*len(vec))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(modelBytes)))
	copy(buf[4:], modelBytes)
	off := 4 + len(modelBytes)
	binary.LittleEndian.PutUint32(buf[off:off+4], uint32(len(vec)))
	copy(buf[off+4:], EncodeVector(vec))
	return buf
}

func decodeInterestCache(b []byte) ([]float32, string, bool) {
	if len(b) < 8 {
		return nil, "", false
	}
	modelLen := int(binary.LittleEndian.Uint32(b[0:4]))
	if modelLen < 0 || 4+modelLen+4 > len(b) {
		return nil, "", false
	}
	model := string(b[4 : 4+modelLen])
	off := 4 + modelLen
	dim := int(binary.LittleEndian.Uint32(b[off : off+4]))
	vec, ok := decodeVector(b[off+4:], dim)
	if !ok {
		return nil, "", false
	}
	return vec, model, true
}

func buildUserVector(signals []interestSignal, embeddings map[uint64][]float32, now time.Time) []float32 {
	if len(signals) == 0 {
		return nil
	}
	vectors := make([][]float32, 0, len(signals))
	weights := make([]float64, 0, len(signals))
	for _, sig := range signals {
		vec, ok := embeddings[sig.VideoID]
		if !ok || len(vec) == 0 {
			continue
		}
		w := sig.Weight * timeDecay(sig.At, now, interestHalfLife)
		if w <= 0 || math.IsNaN(w) {
			continue
		}
		vectors = append(vectors, vec)
		weights = append(weights, w)
	}
	return weightedMean(vectors, weights)
}

// InvalidateUser 在点赞/打赏后重算并写回 MySQL，供推荐和后续推送使用。
func (s *Service) InvalidateUser(accountID uint64) {
	if s == nil || accountID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := s.refreshUserVector(ctx, accountID); err != nil {
		slog.Warn("refresh user embedding failed",
			slog.Uint64("account_id", accountID), slog.String("error", err.Error()))
	}
}

func (s *Service) userVector(ctx context.Context, accountID uint64) ([]float32, error) {
	if accountID == 0 {
		return nil, nil
	}
	model := s.modelName()
	if cached, ok := s.interest.get(ctx, accountID, model); ok {
		return cached, nil
	}
	if stored, err := s.loadStoredUserVector(accountID, model); err != nil {
		return nil, err
	} else if len(stored) > 0 {
		s.interest.set(ctx, accountID, model, stored)
		return stored, nil
	}
	return s.refreshUserVector(ctx, accountID)
}

func (s *Service) loadStoredUserVector(accountID uint64, model string) ([]float32, error) {
	row, err := s.repo.FindUserEmbedding(accountID)
	if err != nil {
		return nil, fmt.Errorf("load user embedding: %w", err)
	}
	if row == nil {
		return nil, nil
	}
	if model != "" && row.Model != model {
		return nil, nil
	}
	vec, ok := decodeVector(row.Vector, row.Dim)
	if !ok {
		return nil, nil
	}
	return vec, nil
}

func (s *Service) refreshUserVector(ctx context.Context, accountID uint64) ([]float32, error) {
	model := s.modelName()
	signals, err := s.repo.ListInterestSignals(accountID, maxInterestItems)
	if err != nil {
		return nil, fmt.Errorf("list interest signals: %w", err)
	}
	if len(signals) == 0 {
		if err := s.repo.DeleteUserEmbedding(accountID); err != nil {
			return nil, err
		}
		s.interest.InvalidateUser(accountID)
		return nil, nil
	}
	ids := make([]uint64, 0, len(signals))
	for _, sig := range signals {
		ids = append(ids, sig.VideoID)
	}
	embeddings, err := s.repo.LoadEmbeddings(ids, model)
	if err != nil {
		return nil, fmt.Errorf("load interest embeddings: %w", err)
	}
	vec := buildUserVector(signals, embeddings, time.Now())
	if len(vec) == 0 {
		// 作品向量还没算出来时保留旧行，避免把已有兴趣档抹掉。
		return s.loadStoredUserVector(accountID, model)
	}
	if err := s.repo.UpsertUserEmbedding(UserEmbedding{
		AccountID: accountID,
		Model:     model,
		Dim:       len(vec),
		Vector:    EncodeVector(vec),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("upsert user embedding: %w", err)
	}
	s.interest.set(ctx, accountID, model, vec)
	return vec, nil
}
