---
title: admin-ui
status: active
code:
  - frontend/src/views/admin/AdminShell.vue
related:
  - frontend/src/views/admin/AdminOverviewView.vue
  - frontend/src/views/admin/AdminReportsView.vue
  - frontend/src/views/admin/AdminVideosView.vue
  - frontend/src/views/admin/AdminUsersView.vue
  - frontend/src/views/admin/AdminOpsView.vue
  - frontend/src/api/admin.ts
  - frontend/src/api/report.ts
  - frontend/src/views/AccountView.vue
  - frontend/src/router/index.ts
  - frontend/src/App.vue
---
# admin-ui

## raw source
The web application gives reviewer accounts a separate administration workbench for the report queue, video lookup and takedown, account lookup, and read-only operations.

## expanded spec
The administration surface is reached from the signed-in account hub, not from the primary consumer navigation. Visitors without reviewer access do not see the entry and cannot stay on the route. The workbench uses its own chrome: an overview, a report queue, a video lookup, an account lookup, and operations. It does not reuse the consumer shell. Operations behavior is owned by `ops-ui`.

The report queue shows outstanding notices grouped by video, with reason totals and sample explanations, then lets the reviewer dismiss the notices or remove the video. Removal from the queue or from the video lookup asks for a written reason and a confirmation. Account lookup accepts an identifier, a username, or an email and lists that author's works in every review state so the reviewer can open them. Share-code paste recognition is disabled on this surface so it cannot hijack an operator typing an identifier.
