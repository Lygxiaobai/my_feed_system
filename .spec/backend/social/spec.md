---
title: social
status: active
code:
  - backend/internal/social/service.go
related:
  - backend/internal/social/handler.go
  - backend/internal/social/repo.go
  - backend/internal/social/entity.go
---
# social

## raw source
Users can follow and unfollow other users and query follower or following relationships.

## expanded spec
A user cannot follow themself, a relationship is unique, and follow state is reflected consistently in the API and the following feed. Repeated follow or unfollow requests remain idempotent from the user's perspective.

Each account carries a follower count that stays consistent with the relationship table: it changes only when a relationship is actually created or removed, never goes negative, and is the value the following feed uses to decide whether an author is delivered by write fanout or read at request time. A follow or unfollow also invalidates the reader's cached following state so the next following-feed read reflects the new relationship, including the newly followed author's earlier videos. A follow that actually creates a relationship also writes a follow notice for the vlogger; unfollow does not retract it. That inbox behavior is owned by `notification/spec.md`.
