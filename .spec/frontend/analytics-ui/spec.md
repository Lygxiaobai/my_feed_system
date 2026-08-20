---
title: analytics-ui
status: active
code:
  - frontend/src/analytics/track.ts
related:
  - frontend/src/analytics/watch.ts
  - frontend/src/main.ts
  - frontend/src/components/AppShell.vue
  - frontend/src/components/VideoPlayer.vue
  - frontend/src/views/HomeView.vue
  - frontend/src/views/VideoDetailView.vue
  - frontend/src/views/VideoView.vue
  - frontend/src/views/AccountView.vue
  - frontend/src/views/RegisterView.vue
  - frontend/src/views/SettingsView.vue
  - frontend/src/router/index.ts
---
# analytics-ui

## raw source
The web application records product behaviors that explain how people move through feed, playback, publishing, search, and account flows.

## expanded spec
A single client helper owns queueing, visitor identity, and delivery. Views do not call the report endpoint themselves. Failed delivery never changes the user-visible result of the action that produced the event.

Route changes emit `page_view`. Search, login, register, logout, publish, like, unlike, follow, unfollow, and comment submit emit only after the corresponding user action succeeds. Feed and detail playback emit `video_play` when playback actually starts and `video_watch` when the user leaves that video, including watch duration when the player can provide it.

The helper batches events and flushes on a short delay, when the batch is full, or when the page is hidden, so playback does not create one request per tick. The visitor identifier is created locally and reused across sessions on the same browser; it is not a login credential.
