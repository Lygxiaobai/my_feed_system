---
title: likes
status: active
code:
  - backend/internal/like/service.go
related:
  - backend/internal/like/handler.go
  - backend/internal/like/repo.go
  - backend/internal/like/entity.go
---
# likes

## raw source
A user can like, unlike, and query the like state of a video without creating duplicate relationships.

## expanded spec
Like and unlike write the relationship and `videos.likes_count` in the same MySQL transaction. The unique relationship key makes concurrent duplicate requests idempotent, so a successful response means the relationship query is already consistent.

After the transaction commits, an outbox event asynchronously updates popularity projections and cache invalidation. A broker or Redis failure must not make the committed like relationship disappear.
