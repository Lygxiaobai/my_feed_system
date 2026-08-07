---
title: video-ui
status: active
code:
  - frontend/src/views/VideoDetailView.vue
  - frontend/src/components/VideoPlayer.vue
related:
  - frontend/src/views/HomeView.vue
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

The shared video player owns playback lifecycle behavior for feed and detail surfaces: muted autoplay, active-item pause rules, loading and buffering feedback, playback errors with retry, and resource cleanup when a video is no longer active. Feed playback keeps only the active item and its immediate neighbors enabled.
