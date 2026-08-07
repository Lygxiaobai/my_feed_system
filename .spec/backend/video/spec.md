---
title: video
status: active
code:
  - backend/internal/video/service.go
related:
  - backend/internal/video/handler.go
  - backend/internal/video/repo.go
  - backend/internal/video/entity.go
  - backend/internal/video/detail_cache.go
  - backend/internal/video/detail_cache_payload.go
  - backend/internal/video/local_detail_cache.go
  - backend/internal/video/media_validator.go
  - backend/internal/video/invalidation_consumer.go
---
# video

## raw source
The video subsystem validates uploads, publishes videos, serves details, and exposes author and liked-video views.

## expanded spec
Video publishing is authenticated and idempotent. Media paths returned by the API remain usable through the static resource surface. Detail reads may use cache but must preserve the persisted video's visible fields. Publishing emits the events required by timeline and popularity consumers.
