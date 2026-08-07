---
title: frontend
status: active
code:
  - frontend/src/main.ts
related:
  - frontend/src/App.vue
  - frontend/src/components/AppShell.vue
  - frontend/src/router/index.ts
  - frontend/src/api/client.ts
  - frontend/src/stores/auth.ts
  - frontend/src/stores/social.ts
  - frontend/src/stores/toast.ts
  - frontend/src/style.css
desc: Vue web application bootstrap and cross-view architecture boundary.
---
# frontend

## raw source
The frontend is a Vue application with a single bootstrap entry, client-side routing, Pinia stores, API modules, and behavior-facing views for feed, video, account, and interactions.

## expanded spec
`frontend/src/main.ts` owns application bootstrap: it creates the Vue application, installs Pinia and the router, loads global styling, and mounts the root component. `frontend/src/App.vue` remains the root composition boundary and delegates page selection to the router.

`frontend/src/router/index.ts` owns the URL-to-view contract. Views own user workflows, API modules own HTTP calls and response types, and Pinia stores own state shared across views. Authentication state has one client-side owner in `stores/auth.ts`; account and interaction flows must not maintain competing token state.

The child nodes own the detailed feed, video, account, and interaction contracts. This parent spec owns application startup, routing, cross-view state boundaries, and the rule that capability behavior remains in its owning child node.

## change rules
Adding or changing a route requires checking the relevant UI spec and its `eval.md`. Changing bootstrap, router installation, global state ownership, or authentication state requires checking this parent spec and every affected child UI spec. A visual refactor that preserves user-visible behavior does not require a new scenario.
