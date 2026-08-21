---
title: interaction-ui
status: active
code:
  - frontend/src/views/HomeView.vue
  - frontend/src/views/VideoDetailView.vue
related:
  - frontend/src/components/TipSheet.vue
  - frontend/src/components/ShareSheet.vue
  - frontend/src/components/ReportSheet.vue
  - frontend/src/components/DanmakuLayer.vue
  - frontend/src/components/AppShell.vue
  - frontend/src/components/FeedVideoCard.vue
  - frontend/src/views/ShareLandingView.vue
  - frontend/src/api/like.ts
  - frontend/src/api/comment.ts
  - frontend/src/api/social.ts
  - frontend/src/api/wallet.ts
  - frontend/src/api/report.ts
  - frontend/src/api/danmaku.ts
  - frontend/src/api/video.ts
  - frontend/src/stores/auth.ts
  - frontend/src/stores/social.ts
  - frontend/src/stores/toast.ts
  - frontend/src/views/NotificationsView.vue
  - frontend/src/components/NotificationPanel.vue
---
# interaction-ui

## raw source
The web application exposes like, comment, follow, unfollow, tipping, a signed-in video tip list, sharing, reporting, timed danmaku, and the resulting inbox notices from the supported video surfaces and keeps their visible state consistent with API responses.

## expanded spec
Inbox presentation of those actions is owned by `notification-ui`. Repeated actions remain understandable and do not produce contradictory local state. Loading, success, and failure feedback is visible at the action's surface, and a successful mutation updates the relevant video or profile view. The tip list on a video is available to every signed-in user: the author sees all tips, and a non-author sees only their own rows.

Like mutations remain pending until the API's transactional response succeeds; the UI does not wait for asynchronous popularity projection. Comment requests are applied only when their response still belongs to the open video, and comment loading, refresh, submit, and delete states do not incorrectly block one another.

Danmaku is a playback overlay, not the comment drawer. When it is on, stored items fly across the video at the playback offset they were sent, the animation pauses with the video, and looping the video replays them. Opening or seeking into the middle of a video shows only the items that would still be crossing the screen at that moment, including when the list arrives after playback has already started. A signed-in viewer sends from a short input on the playback surface; the text appears immediately and stays only if the send succeeds. The overlay preference persists on the device. Mute and the danmaku on/off control sit in that same composer bar, so turning the overlay off hides the flying text but leaves the bar. A control whose own label already shows on or off does not also raise a transient message.

Feedback is reserved for outcomes the user cannot otherwise observe. A control whose own label already reflects the resulting state does not additionally raise a transient message. Playback surfaces carry no persistent instructional overlay describing available gestures or keyboard shortcuts; those inputs remain supported but are not advertised on top of the video.

Sharing produces a passage of text containing a code, the author and title, and a link, and the user copies the whole passage. The link prefix follows the entry point the user is currently on, because the site is reachable through more than one origin and a share that names the wrong one is useless to the recipient. Copying degrades to a manual-copy prompt when the clipboard is unavailable, which is the normal case on the non-secure entry point rather than an edge case. The code itself is displayed, so a user can read it out or retype it.

The recipient redeems a share by pasting into the search field, which is where a user naturally puts copied text. The application never reads the clipboard on its own: doing so requires a secure context that one of the entry points does not provide. Pasted text that unambiguously carries a code navigates to the video, and a failure to resolve it is reported rather than silently searched for, because searching for a whole share passage returns nothing meaningful. Text that merely resembles a code falls back to an ordinary search when it does not resolve, so a real search term is never swallowed. A share link opened directly resolves and lands on the video without leaving the intermediate step in history.

Reporting is offered on content the viewer does not own and requires choosing from the offered reasons; the catch-all reason requires an explanation before the form can be submitted. The interface states before and after submission that the content is not removed immediately and that a person will review it, because a report produces no observable change and a user who expects one will assume the submission failed and repeat it.
