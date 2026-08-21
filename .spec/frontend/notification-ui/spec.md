---
title: notification-ui
status: active
code:
  - frontend/src/views/NotificationsView.vue
related:
  - frontend/src/components/NotificationPanel.vue
  - frontend/src/components/AppShell.vue
  - frontend/src/api/notification.ts
  - frontend/src/stores/notification.ts
  - frontend/src/router/index.ts
---
# notification-ui

## raw source
Signed-in users open interactive notices from the top-bar bell and a dedicated notifications page, filtered like a Douyin inbox.

## expanded spec
The desktop bell opens a dropdown over the current page instead of navigating away. The compact top bar keeps a bell that goes to `/notifications`. A signed-out click on either control goes to the account page and does not call the inbox API. The dedicated page shows the same list for a signed-in user and asks the visitor to sign in otherwise.

Each row shows the actor, a type mark on the avatar, the action and a relative time, optional comment text, and a cover for the related video. Follow rows that the viewer does not already follow offer a follow-back control. Filters cover all notices, follows, likes, mentions, comments-and-replies, and tips. Opening a row marks it read and goes to the video or the actor's profile. The unread badge on the bell tracks the inbox unread count while the session is signed in.

## change rules
Changing the inbox filters, destinations, or when the badge updates requires checking `backend/notification`. Changing the top-bar placement of the bell requires checking the frontend parent spec and `account-ui`.
