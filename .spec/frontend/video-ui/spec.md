---
title: video-ui
status: active
code:
  - frontend/src/views/VideoDetailView.vue
  - frontend/src/components/VideoPlayer.vue
  - frontend/src/views/VideoView.vue
related:
  - frontend/src/views/HomeView.vue
  - frontend/src/api/video.ts
  - frontend/src/api/client.ts
  - frontend/src/components/AppIcon.vue
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/router/index.ts
---
# video-ui

## raw source
Users can upload, process, publish, open, and play a video, and move between video-related workflows without losing the selected video identity.

## expanded spec
Upload processing, task failure, detail loading, media URLs, missing videos, and navigation failures have explicit UI states. The publish action waits for the server task to become ready before sending playable URLs. Actions originating from a detail view refresh or invalidate the visible data instead of leaving a contradictory stale card.

The publish workflow is file-first. Selecting or dropping an acceptable video starts the upload immediately so the title and description can be edited in parallel. Those fields stay editable until the publish request is sent. Selecting a file does not copy the file name into the title; the user writes the title themselves, and an empty title keeps publish unavailable. A ready media task is reused: clicking publish does not upload the same file again. If the user clicks publish while upload or processing is still running, the workflow waits for ready URLs and then publishes once. Replacing or clearing the file cancels any in-flight work. Leaving the page during upload, processing, publishing, or an unpublished ready task warns the user; confirming the leave cancels unfinished upload work. After a successful publish the form is replaced by a completion state with a play action and a start-over action. An unsigned visitor sees a login prompt instead of the composer.

The upload workflow rejects an unusable file before any network request: a non-video type or a file above the server's size limit fails at selection or drop time with the reason shown. An accepted upload reports real byte-level progress while the request body is sent, but never presents 100% until the server has accepted the file. After the browser finishes sending bytes, the UI shows a confirming state while waiting for the server response, then reports processing and publishing as distinct in-progress states, and stays cancellable throughout. Cancellation is a normal outcome, not a failure state. In-progress state is described in user terms and does not expose transport, worker, or transcoding internals.

The video poster is derived by the server from the video's first frame. It is not a user-facing concept: no cover is selected, uploaded, previewed, or confirmed anywhere in the publish workflow.

The shared video player owns playback lifecycle behavior for feed and detail surfaces: muted autoplay, active-item pause rules, loading and buffering feedback, playback errors with retry, and resource cleanup when a video is no longer active. It exposes play, pause, seek, and playback-time updates so overlays on the same surface can stay in sync. The active player shows a play/pause control on desktop, current time and total duration as `mm:ss / mm:ss`, and a thin seekable progress bar along the bottom edge; dragging or tapping the bar moves playback to that offset. Feed playback keeps only the active item and its immediate neighbors enabled. On desktop feed and detail pages the picture fills a dark rounded stage with a short gutter, still cropping rather than letterboxing; on compact screens the stage goes edge to edge. Detail playback may seek to an unfinished history position after metadata is ready; the feed does not auto-seek from history. Progress reporting for that history is owned by `history-ui`.
