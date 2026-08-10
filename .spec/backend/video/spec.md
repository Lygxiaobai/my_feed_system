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
  - backend/internal/media/entity.go
  - backend/internal/media/repo.go
  - backend/internal/media/service.go
  - backend/internal/media/processor.go
  - backend/internal/worker/media_worker.go
---
# video

## raw source
The video subsystem validates uploads, publishes videos, serves details, and exposes author and liked-video views.

## expanded spec
Video publishing is authenticated and idempotent. A video upload creates an account-owned `processing` media task and returns no playable URL until the Worker has produced a standard MP4 and JPEG poster. Tasks finish as `ready` with `/static/videos/*.mp4` and `/static/covers/*.jpg` URLs or as `failed` with a bounded error message.

The Worker uses ffmpeg to normalize video codec/container and MIME-by-extension, enables MP4 fast start, and generates the poster. Raw source files remain private to the shared upload volume and are not exposed by the static resource surface. Media paths returned by the API remain usable through the static resource surface. Detail reads may use cache but must preserve the persisted video's visible fields. Publishing accepts only ready, playable media and emits the events required by timeline and popularity consumers.
