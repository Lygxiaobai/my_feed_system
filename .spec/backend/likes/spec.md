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
Like writes preserve relationship uniqueness and keep the visible like state consistent with the asynchronous or synchronous persistence path. A like change invalidates or refreshes the relevant video and ranking data.
