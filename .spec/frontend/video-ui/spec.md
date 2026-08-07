---
title: video-ui
status: active
code:
  - frontend/src/views/VideoDetailView.vue
related:
  - frontend/src/views/VideoView.vue
  - frontend/src/api/video.ts
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/router/index.ts
---
# video-ui

## raw source
Users can open a video, view its metadata and media, and move between video-related workflows without losing the selected video identity.

## expanded spec
Detail loading, media URLs, missing videos, and navigation failures have explicit UI states. Actions originating from a detail view refresh or invalidate the visible data instead of leaving a contradictory stale card.
