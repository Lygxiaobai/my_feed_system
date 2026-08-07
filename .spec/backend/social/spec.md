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
