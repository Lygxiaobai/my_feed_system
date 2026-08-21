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
---
# feed

## raw source
The backend serves latest, following, likes-count, and popularity feeds with stable pagination and authenticated access where required.

## expanded spec
Feed reads prefer the appropriate Redis timeline, ranking, or page cache and can fall back to MySQL when a cache is unavailable or a ranking has no usable entries. Popularity fallback results use the persisted MySQL popularity score and set as_of to zero. Cursor or snapshot pagination must not duplicate or skip items across adjacent pages. Following feed results are scoped to the authenticated account. Cache invalidation and asynchronous timeline updates must not make newly published videos permanently invisible.

The following feed combines write fanout and read fanout. Videos from authors below the configured follower threshold are delivered into a per-user inbox when they are published, videos from authors at or above the pull threshold stay in a per-author outbox and are merged at read time, and an author between the two thresholds is delivered only to followers whose inbox is currently maintained. A read must produce the same items regardless of which side delivered them: results are restricted to the reader's current following set, so an unfollowed author's residue in an inbox is never returned, and a reader whose inbox is not currently maintained has it rebuilt from MySQL before the page is assembled. A page may only report that no further pages exist when the inbox and every merged outbox are known to be complete; otherwise the page is produced from MySQL. When Redis, the inbox, or the outbox is unavailable, the following feed degrades to reading MySQL directly and remains correct.
