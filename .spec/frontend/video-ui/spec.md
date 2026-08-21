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
  - frontend/src/components/AppIcon.vue
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/router/index.ts
---
# video-ui

## raw source
Users can upload, process, publish, open, and play a video, and move between video-related workflows without losing the selected video identity.

## expanded spec
Upload processing, task failure, detail loading, media URLs, missing videos, and navigation failures have explicit UI states. The publish action waits for the server task to become ready before sending playable URLs. Actions originating from a detail view refresh or invalidate the visible data instead of leaving a contradictory stale card.

The upload workflow rejects an unusable file before any network request: a non-video type or a file above the server's size limit fails at selection time with the reason shown. An accepted upload reports real byte-level progress while the request body is sent, then reports processing and publishing as distinct in-progress states, and stays cancellable throughout. Cancellation is a normal outcome, not a failure state. In-progress state is described in user terms and does not expose transport, worker, or transcoding internals.

The video poster is derived by the server from the video's first frame. It is not a user-facing concept: no cover is selected, uploaded, previewed, or confirmed anywhere in the publish workflow.

The shared video player owns playback lifecycle behavior for feed and detail surfaces: muted autoplay, active-item pause rules, loading and buffering feedback, playback errors with retry, and resource cleanup when a video is no longer active. It exposes play, pause, seek, and playback-time updates so overlays on the same surface can stay in sync. The active player shows current time and total duration as `mm:ss / mm:ss` and a thin seekable progress bar; dragging or tapping the bar moves playback to that offset. Feed playback keeps only the active item and its immediate neighbors enabled. On feed and detail pages the video fills the remaining chrome, cropping rather than letterboxing inside a smaller card. Detail playback may seek to an unfinished history position after metadata is ready; the feed does not auto-seek from history. Progress reporting for that history is owned by `history-ui`.
