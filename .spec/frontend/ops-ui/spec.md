---
title: ops-ui
status: active
code:
  - frontend/src/views/OpsView.vue
related:
  - frontend/src/api/ops.ts
  - frontend/src/views/AccountView.vue
  - frontend/src/router/index.ts
  - frontend/nginx.conf
---
# ops-ui

## raw source
The web application gives test-email accounts an operations surface with a Grafana dashboard and a Loki log search.

## expanded spec
The operations surface is reached from the signed-in account hub, not from the primary navigation. Visitors without a test-email identity do not see the entry and cannot stay on the route.

The monitor tab embeds the existing overview dashboard through the site's Grafana path. The logs tab sends a LogQL query and shows the returned lines. Failed lookups show the server message. The page does not display Grafana usernames or passwords.
