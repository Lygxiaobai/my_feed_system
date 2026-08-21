---
title: interaction-ui
status: active
code:
  - frontend/src/views/HomeView.vue
  - frontend/src/views/VideoDetailView.vue
related:
  - frontend/src/components/TipSheet.vue
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/api/like.ts
  - frontend/src/api/comment.ts
  - frontend/src/api/social.ts
  - frontend/src/api/wallet.ts
  - frontend/src/stores/auth.ts
  - frontend/src/stores/social.ts
  - frontend/src/stores/toast.ts
---
# interaction-ui

## raw source
The web application exposes like, comment, follow, unfollow, tipping, and a signed-in video tip list from the supported video surfaces and keeps their visible state consistent with API responses.

## expanded spec
Repeated actions remain understandable and do not produce contradictory local state. Loading, success, and failure feedback is visible at the action's surface, and a successful mutation updates the relevant video or profile view. The tip list on a video is available to every signed-in user: the author sees all tips, and a non-author sees only their own rows.

Like mutations remain pending until the API's transactional response succeeds; the UI does not wait for asynchronous popularity projection. Comment requests are applied only when their response still belongs to the open video, and comment loading, refresh, submit, and delete states do not incorrectly block one another.

Feedback is reserved for outcomes the user cannot otherwise observe. A control whose own label already reflects the resulting state does not additionally raise a transient message. Playback surfaces carry no persistent instructional overlay describing available gestures or keyboard shortcuts; those inputs remain supported but are not advertised on top of the video.
