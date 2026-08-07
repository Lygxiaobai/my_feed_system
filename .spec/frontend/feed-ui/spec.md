---
title: feed-ui
status: active
code:
  - frontend/src/views/HomeView.vue
related:
  - frontend/src/api/feed.ts
  - frontend/src/api/types.ts
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/views/HotView.vue
  - frontend/src/router/index.ts
  - frontend/src/stores/auth.ts
---
# feed-ui

## raw source
The web application lets a user browse recommendation, likes-count, following, and hot-ranking feeds with loading, pagination, authentication, and error states.

## expanded spec
The UI preserves the active feed mode and cursor while loading adjacent pages, renders stable video cards, and reports API failures without silently showing stale or empty content as success. Authenticated-only feed modes redirect or explain the missing session state.
