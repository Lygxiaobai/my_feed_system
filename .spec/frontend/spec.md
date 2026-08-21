---
title: frontend
status: active
code:
  - frontend/src/main.ts
related:
  - frontend/src/App.vue
  - frontend/src/components/ShareInbox.vue
  - frontend/src/components/AppIcon.vue
  - frontend/src/components/AppShell.vue
  - frontend/src/router/index.ts
  - frontend/src/api/client.ts
  - frontend/src/analytics/track.ts
  - frontend/src/stores/auth.ts
  - frontend/src/stores/social.ts
  - frontend/src/stores/toast.ts
  - frontend/src/style.css
  - frontend/src/views/NotificationsView.vue
desc: Vue web application bootstrap and cross-view architecture boundary.
---
# frontend

## raw source
The frontend is a Vue application with a single bootstrap entry, client-side routing, Pinia stores, API modules, and behavior-facing views for feed, video, account, wallet, check-in, lottery, interactions, and notifications.

## expanded spec
`frontend/src/main.ts` owns application bootstrap: it creates the Vue application, installs Pinia and the router, loads global styling, and mounts the root component. `frontend/src/App.vue` remains the root composition boundary and delegates page selection to the router. Share recognition is mounted here so a paste or clipboard offer survives navigation and does not depend on a particular view wrapping `AppShell`.

`frontend/src/router/index.ts` owns the URL-to-view contract. Views own user workflows, API modules own HTTP calls and response types, and Pinia stores own state shared across views. Authentication state has one client-side owner in `stores/auth.ts`; account and interaction flows must not maintain competing token state.

`api/client.ts` owns the wire contract. It unwraps the backend's response envelope in one place, so API modules and their types describe only the payload and never the envelope. A non-success business code becomes a thrown error carrying the backend's user-facing message, its business code, and the request identifier; views display that message rather than composing their own wording for server-side failures. Because unwrapping is centralized, adding or changing an endpoint does not require each caller to re-implement success detection.

`components/AppShell.vue` owns the navigation surface. Primary destinations stay on the desktop sidebar and the compact bottom navigation; there is no overflow menu duplicating those destinations. The desktop top bar, aligned with search, also hosts utility entries: 充积分, a client download placeholder, a wallpaper placeholder, notifications, messages, publish, and the account avatar. Entries that already have an API open that page or panel; the rest open a frontend-only placeholder that does not invent a backend. The notifications bell opens the inbox owned by `notification-ui`. Signed-out visitors who hit an authenticated utility are sent to the account page. The compact "mine" destination still resolves to the account hub. The chrome shows no routing or diagnostic identifiers.

The child nodes own the detailed feed, video, account, wallet, interaction, notification, and analytics contracts. This parent spec owns application startup, routing, cross-view state boundaries, and the rule that capability behavior remains in its owning child node.

## change rules
Adding or changing a route requires checking the relevant UI spec and its `eval.md`. Changing bootstrap, router installation, global state ownership, or authentication state requires checking this parent spec and every affected child UI spec. A visual refactor that preserves user-visible behavior does not require a new scenario.
