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
  - backend/internal/feed/invalidation_consumer.go
---
# feed

## raw source
The backend serves latest, following, likes-count, and popularity feeds with stable pagination and authenticated access where required.

## expanded spec
Feed reads prefer the appropriate Redis timeline, ranking, or page cache and can fall back to MySQL when a cache is unavailable. Cursor or snapshot pagination must not duplicate or skip items across adjacent pages. Following feed results are scoped to the authenticated account. Cache invalidation and asynchronous timeline updates must not make newly published videos permanently invisible.
