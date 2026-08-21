---
title: feed-ui
status: active
code:
  - frontend/src/views/HomeView.vue
related:
  - frontend/src/api/feed.ts
  - frontend/src/api/types.ts
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/components/VideoPlayer.vue
  - frontend/src/views/HotView.vue
  - frontend/src/router/index.ts
  - frontend/src/stores/auth.ts
---
# feed-ui

## raw source
The web application lets a user browse recommendation, likes-count, following, and hot-ranking feeds with loading, pagination, authentication, and error states.

## expanded spec
The UI preserves the active feed mode and cursor while loading adjacent pages, renders stable video cards, and reports API failures without silently showing stale or empty content as success. Authenticated-only feed modes redirect or explain the missing session state. The feed chrome does not offer a separate detail button; comments, follow, share, tipping, a signed-in tip-list control, and the danmaku overlay stay on the playback surface. List cards on the hot-ranking view also omit a playback-URL control and a separate details/comments chip, because the cover and title already open the video. Any signed-in user can open that video's tip list: authors see every tip, viewers see only their own.
