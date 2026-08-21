package feed

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// 关注流推拉结合用到的 Redis key。
//
// 收件箱与发件箱都是 ZSET，member 为 videoID、score 为发布时间毫秒，
// 与 feed:global_timeline 保持同一套编码，读路径可以共用游标与回填逻辑。
const (
	inboxKeyPrefix     = "feed:inbox:"
	inboxActiveKeyPfx  = "feed:inbox:active:"
	outboxKeyPrefix    = "feed:outbox:"
	followingKeyPrefix = "feed:following:"
)

const (
	defaultInboxMaxSize  = int64(1000)
	defaultOutboxMaxSize = int64(200)
	defaultActiveTTL     = 7 * 24 * time.Hour
	// 关注列表变化后由写路径主动删除 key，TTL 只是兜底，不需要很短。
	defaultFollowingCacheTTL = 5 * time.Minute
)

// TimelineEntry 是收件箱与发件箱里的一条时间线记录。
type TimelineEntry struct {
	VideoID   uint64
	CreatedAt time.Time
}

// InboxStore 维护每个用户的关注流收件箱，是写扩散的落点。
//
// 除了收件箱本身，它还负责「活跃标记」。两者刻意用同一套生命周期：
// 标记存在即代表该用户的收件箱正在被扩散维护，可以信任；标记不存在
// 则说明用户长期没读过关注流、扩散早已跳过他，读路径必须回源重建。
// 合成一个概念之后，冷启动、Redis 被淘汰、长期不活跃这三种情况
// 走的是同一条恢复路径，不需要各自的补偿逻辑。
type InboxStore struct {
	client    redis.Cmdable
	maxSize   int64
	activeTTL time.Duration
}

// NewInboxStore 创建收件箱存储。maxSize 与 activeTTL 非正时使用默认值。
func NewInboxStore(client redis.Cmdable, maxSize int64, activeTTL time.Duration) *InboxStore {
	if maxSize <= 0 {
		maxSize = defaultInboxMaxSize
	}
	if activeTTL <= 0 {
		activeTTL = defaultActiveTTL
	}

	return &InboxStore{
		client:    client,
		maxSize:   maxSize,
		activeTTL: activeTTL,
	}
}

func (s *InboxStore) Enabled() bool {
	return s != nil && s.client != nil
}

// MaxSize 返回收件箱容量。读路径用它判断收件箱是否已被截断。
func (s *InboxStore) MaxSize() int64 {
	if s == nil {
		return defaultInboxMaxSize
	}
	return s.maxSize
}

func (s *InboxStore) key(userID uint64) string {
	return inboxKeyPrefix + strconv.FormatUint(userID, 10)
}

func (s *InboxStore) activeKey(userID uint64) string {
	return inboxActiveKeyPfx + strconv.FormatUint(userID, 10)
}

// IsActive 判断某个用户的收件箱是否处于被维护状态。
func (s *InboxStore) IsActive(ctx context.Context, userID uint64) (bool, error) {
	if !s.Enabled() || userID == 0 {
		return false, nil
	}

	count, err := s.client.Exists(ctx, s.activeKey(userID)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkActive 标记用户收件箱进入维护状态并续期。
func (s *InboxStore) MarkActive(ctx context.Context, userID uint64) error {
	if !s.Enabled() || userID == 0 {
		return nil
	}
	return s.client.Set(ctx, s.activeKey(userID), 1, s.activeTTL).Err()
}

// ClearActive 让下一次读取强制回源重建收件箱。
// 关注关系变化后调用：此时收件箱相对新的关注列表已经不完整，
// 与其精确增删，不如让读路径按最新关系重建一次。
func (s *InboxStore) ClearActive(ctx context.Context, userID uint64) error {
	if !s.Enabled() || userID == 0 {
		return nil
	}
	return s.client.Del(ctx, s.activeKey(userID)).Err()
}

// FilterActive 从候选粉丝中挑出仍在维护收件箱的那部分，用于中V 的分级推送。
func (s *InboxStore) FilterActive(ctx context.Context, userIDs []uint64) ([]uint64, error) {
	if !s.Enabled() || len(userIDs) == 0 {
		return nil, nil
	}

	cmds := make([]*redis.IntCmd, 0, len(userIDs))
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, userID := range userIDs {
			cmds = append(cmds, pipe.Exists(ctx, s.activeKey(userID)))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	active := make([]uint64, 0, len(userIDs))
	for i, cmd := range cmds {
		if cmd.Val() > 0 {
			active = append(active, userIDs[i])
		}
	}
	return active, nil
}

// Push 把一个视频写扩散到一批粉丝的收件箱。
//
// ZADD 用相同 member 与 score 重复写没有副作用，因此扩散消息重投是天然幂等的，
// 不需要 processed_messages 那套去重。
func (s *InboxStore) Push(ctx context.Context, userIDs []uint64, videoID uint64, createdAt time.Time) error {
	if !s.Enabled() || len(userIDs) == 0 || videoID == 0 {
		return nil
	}

	item := redis.Z{
		Score:  float64(createdAt.UnixMilli()),
		Member: strconv.FormatUint(videoID, 10),
	}
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, userID := range userIDs {
			if userID == 0 {
				continue
			}
			key := s.key(userID)
			pipe.ZAdd(ctx, key, item)
			pipe.ZRemRangeByRank(ctx, key, 0, -s.maxSize-1)
		}
		return nil
	})
	return err
}

// Fill 把一批记录合并进某个用户的收件箱，用于回源重建。
//
// 刻意不先 DEL：删除与并发扩散之间存在窗口，会把刚推进来的新视频抹掉。
// 合并写入配合读路径按关注列表过滤，既没有丢失风险，也不会漏掉已取关的清理。
func (s *InboxStore) Fill(ctx context.Context, userID uint64, entries []TimelineEntry) error {
	if !s.Enabled() || userID == 0 || len(entries) == 0 {
		return nil
	}

	items := make([]redis.Z, 0, len(entries))
	for _, entry := range entries {
		if entry.VideoID == 0 {
			continue
		}
		items = append(items, redis.Z{
			Score:  float64(entry.CreatedAt.UnixMilli()),
			Member: strconv.FormatUint(entry.VideoID, 10),
		})
	}
	if len(items) == 0 {
		return nil
	}

	key := s.key(userID)
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, key, items...)
		pipe.ZRemRangeByRank(ctx, key, 0, -s.maxSize-1)
		return nil
	})
	return err
}

// ListVideoIDs 按时间游标从收件箱读取一批候选视频 ID，同时返回收件箱当前条数。
// 条数用于判断收件箱是否已达容量上限——未满即代表内容完整。
func (s *InboxStore) ListVideoIDs(ctx context.Context, userID uint64, latestTime int64, count int64) ([]uint64, int64, error) {
	if !s.Enabled() || userID == 0 {
		return nil, 0, nil
	}

	key := s.key(userID)
	var (
		membersCmd *redis.StringSliceCmd
		cardCmd    *redis.IntCmd
	)
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		membersCmd = pipe.ZRevRangeByScore(ctx, key, rangeByScoreCursor(latestTime, count))
		cardCmd = pipe.ZCard(ctx, key)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return parseVideoIDs(membersCmd.Val()), cardCmd.Val(), nil
}

// Remove 从收件箱中移除一批已经失效的成员。
func (s *InboxStore) Remove(ctx context.Context, userID uint64, videoIDs ...uint64) error {
	if !s.Enabled() || userID == 0 {
		return nil
	}

	members := membersOf(videoIDs)
	if len(members) == 0 {
		return nil
	}
	return s.client.ZRem(ctx, s.key(userID), members...).Err()
}

// OutboxStore 维护每个作者的发件箱，是读扩散的来源。
//
// 所有作者都会写发件箱，不只是大V：一方面判定阈值可以随时调整，
// 另一方面粉丝数跨过阈值时不需要为历史内容做任何补偿。
type OutboxStore struct {
	client  redis.Cmdable
	maxSize int64
}

// NewOutboxStore 创建发件箱存储。maxSize 非正时使用默认值。
func NewOutboxStore(client redis.Cmdable, maxSize int64) *OutboxStore {
	if maxSize <= 0 {
		maxSize = defaultOutboxMaxSize
	}
	return &OutboxStore{client: client, maxSize: maxSize}
}

func (s *OutboxStore) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *OutboxStore) key(authorID uint64) string {
	return outboxKeyPrefix + strconv.FormatUint(authorID, 10)
}

// Add 把作者新发布的视频写入其发件箱。
func (s *OutboxStore) Add(ctx context.Context, authorID uint64, videoID uint64, createdAt time.Time) error {
	if !s.Enabled() || authorID == 0 || videoID == 0 {
		return nil
	}

	key := s.key(authorID)
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(createdAt.UnixMilli()),
			Member: strconv.FormatUint(videoID, 10),
		})
		pipe.ZRemRangeByRank(ctx, key, 0, -s.maxSize-1)
		return nil
	})
	return err
}

// ListVideoIDs 按时间游标读取某个作者发件箱里的候选视频 ID。
// 第二个返回值表示本次是否读满，读满意味着更早的内容还在发件箱里没取回来。
func (s *OutboxStore) ListVideoIDs(ctx context.Context, authorID uint64, latestTime int64, count int64) ([]uint64, bool, error) {
	if !s.Enabled() || authorID == 0 {
		return nil, false, nil
	}

	members, err := s.client.ZRevRangeByScore(ctx, s.key(authorID), rangeByScoreCursor(latestTime, count)).Result()
	if err != nil {
		return nil, false, err
	}

	return parseVideoIDs(members), int64(len(members)) >= count, nil
}

// Remove 从发件箱中移除一批已经失效的成员。
func (s *OutboxStore) Remove(ctx context.Context, authorID uint64, videoIDs ...uint64) error {
	if !s.Enabled() || authorID == 0 {
		return nil
	}

	members := membersOf(videoIDs)
	if len(members) == 0 {
		return nil
	}
	return s.client.ZRem(ctx, s.key(authorID), members...).Err()
}

// FollowedAuthor 是关注流读路径需要的作者信息。
type FollowedAuthor struct {
	VloggerID uint64 `json:"vlogger_id"`
	// FollowerCount 决定该作者是否走读扩散，取自 accounts.follower_count。
	FollowerCount int64 `json:"follower_count"`
}

// FollowingCache 缓存用户的关注列表及其粉丝量级。
//
// 读路径必须拿到完整关注列表：一是要挑出走读扩散的大V，
// 二是要把已取关作者残留在收件箱里的视频过滤掉。
type FollowingCache struct {
	client redis.Cmdable
	ttl    time.Duration
}

// NewFollowingCache 创建关注列表缓存。ttl 非正时使用默认值。
func NewFollowingCache(client redis.Cmdable, ttl time.Duration) *FollowingCache {
	if ttl <= 0 {
		ttl = defaultFollowingCacheTTL
	}
	return &FollowingCache{client: client, ttl: ttl}
}

func (c *FollowingCache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *FollowingCache) key(userID uint64) string {
	return followingKeyPrefix + strconv.FormatUint(userID, 10)
}

// Get 读取缓存的关注列表。第二个返回值为 false 表示未命中。
func (c *FollowingCache) Get(ctx context.Context, userID uint64) ([]FollowedAuthor, bool, error) {
	if !c.Enabled() || userID == 0 {
		return nil, false, nil
	}

	payload, err := c.client.Get(ctx, c.key(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var authors []FollowedAuthor
	if err := json.Unmarshal(payload, &authors); err != nil {
		return nil, false, err
	}
	return authors, true, nil
}

// Set 写入关注列表缓存。空列表同样需要缓存，否则未关注任何人的用户每次都要查库。
func (c *FollowingCache) Set(ctx context.Context, userID uint64, authors []FollowedAuthor) error {
	if !c.Enabled() || userID == 0 {
		return nil
	}
	if authors == nil {
		authors = []FollowedAuthor{}
	}

	payload, err := json.Marshal(authors)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.key(userID), payload, c.ttl).Err()
}

// Delete 在关注关系变化后清除缓存。
func (c *FollowingCache) Delete(ctx context.Context, userID uint64) error {
	if !c.Enabled() || userID == 0 {
		return nil
	}
	return c.client.Del(ctx, c.key(userID)).Err()
}

// rangeByScoreCursor 把时间游标翻译成 ZSET 的按分值倒序读取参数。
func rangeByScoreCursor(latestTime int64, count int64) *redis.ZRangeBy {
	rangeBy := &redis.ZRangeBy{
		Max:    "+inf",
		Min:    "-inf",
		Offset: 0,
		Count:  count,
	}
	if latestTime > 0 {
		// 同毫秒内可能有多条，取闭区间后由调用方按 (created_at, id) 精确过滤。
		rangeBy.Max = strconv.FormatInt(latestTime, 10)
	}
	return rangeBy
}

func parseVideoIDs(members []string) []uint64 {
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		videoID, err := strconv.ParseUint(member, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, videoID)
	}
	return ids
}

func membersOf(videoIDs []uint64) []any {
	members := make([]any, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		if videoID == 0 {
			continue
		}
		members = append(members, strconv.FormatUint(videoID, 10))
	}
	return members
}
