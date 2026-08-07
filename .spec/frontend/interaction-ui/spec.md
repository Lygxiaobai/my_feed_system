---
title: interaction-ui
status: active
code:
  - frontend/src/views/HomeView.vue
  - frontend/src/views/VideoDetailView.vue
related:
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/api/like.ts
  - frontend/src/api/comment.ts
  - frontend/src/api/social.ts
  - frontend/src/stores/auth.ts
  - frontend/src/stores/social.ts
  - frontend/src/stores/toast.ts
---
# interaction-ui

## raw source
The web application exposes like, comment, follow, and unfollow actions from feed and video-detail surfaces and keeps their visible state consistent with API responses.

## expanded spec
Repeated actions remain understandable and do not produce contradictory local state. Loading, success, and failure feedback is visible at the action's surface, and a successful mutation updates the relevant video or profile view.
