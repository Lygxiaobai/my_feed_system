---
title: feed
status: active
code:
  - backend/internal/feed/service.go
related:
  - backend/internal/feed/handler.go
  - backend/internal/feed/repo.go
  - backend/internal/feed/entity.go
  - backend/internal/feed/cache.go
  - backend/internal/feed/local_page_cache.go
  - backend/internal/feed/timeline_cache.go
  - backend/internal/feed/fanout_store.go
  - backend/internal/feed/invalidation_consumer.go
  - backend/internal/recommend/service.go
  - backend/internal/recommend/mixer.go
  - backend/internal/recommend/interest.go
  - backend/internal/recommend/embedder.go
  - backend/internal/recommend/repo.go
---
# feed

## raw source
The backend serves latest, following, likes-count, popularity, and personalized recommendation feeds with stable pagination and authenticated access where required. Feed reads share an IP ceiling; recommendation is held to a tighter ceiling because mixing is more expensive than a cache-backed latest page.

## expanded spec
Feed reads prefer the appropriate Redis timeline, ranking, or page cache and can fall back to MySQL when a cache is unavailable or a ranking has no usable entries. Popularity fallback results use the persisted MySQL popularity score and set as_of to zero. Cursor or snapshot pagination must not duplicate or skip items across adjacent pages. Following feed results are scoped to the authenticated account. Cache invalidation and asynchronous timeline updates must not make newly published videos permanently invisible.

首页推荐不再复用热度窗。`listRecommend` 对已过审内容做三队列占位混排：兴趣（标题/描述向量与用户兴趣向量的余弦）、普通人作者、热度探索。一页默认 10 条时普通人槽位占 20%；该队列空时回填兴趣，不回填大 V 热度。热度只在热度队列内部排序。多样性（MMR 与作者窗口）是最后一步。匿名或没有兴趣信号时走热度与普通人/最新的冷启动混排。分页用已出视频 id 排除集，不使用热度快照的 `(as_of, offset)`。未过审内容不得出现。粉丝数阈值只用于推荐侧的普通人判定，与关注流推拉阈值无关。用户兴趣向量由最近打赏与点赞作品的向量加权平均得到，主副本落在 `user_embeddings`，供推荐读取和后续推送使用。点赞或打赏后重算并写回；没有兴趣信号时删除该行。视频向量在过审后异步计算，主副本在 `video_embeddings`。

The following feed combines write fanout and read fanout. Videos from authors below the configured follower threshold are delivered into a per-user inbox when they are published, videos from authors at or above the pull threshold stay in a per-author outbox and are merged at read time, and an author between the two thresholds is delivered only to followers whose inbox is currently maintained. A read must produce the same items regardless of which side delivered them: results are restricted to the reader's current following set, so an unfollowed author's residue in an inbox is never returned, and a reader whose inbox is not currently maintained has it rebuilt from MySQL before the page is assembled. A page may only report that no further pages exist when the inbox and every merged outbox are known to be complete; otherwise the page is produced from MySQL. When Redis, the inbox, or the outbox is unavailable, the following feed degrades to reading MySQL directly and remains correct.
