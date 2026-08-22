---
title: ops-ui
status: active
code:
  - frontend/src/views/admin/AdminOpsView.vue
related:
  - frontend/src/api/ops.ts
  - frontend/src/views/admin/AdminShell.vue
  - frontend/src/router/index.ts
  - frontend/nginx.conf
  - monitoring/grafana/provisioning/dashboards/feed-overview.json
---
# ops-ui

## raw source
The administration workbench gives reviewer accounts an operations surface with a Grafana dashboard and a Loki log search.

## expanded spec
The operations surface lives inside the administration workbench owned by `admin-ui`, not on the account hub and not in the primary navigation. Visitors without reviewer access do not see it and cannot stay on the route. A leftover `/ops` address sends the visitor to that same workbench page. A test-email identity does not open this surface.

The monitor tab embeds the existing overview dashboard through the site's Grafana path. That dashboard aggregates product_event records from the log store: counts by allowed event name, play counts by video, average watch duration, and a formatted recent-event list. It does not keep the product events as an unparsed JSON log wall. The logs tab sends a LogQL query and shows the returned lines. Failed lookups show the server message. The page does not display Grafana usernames or passwords.
