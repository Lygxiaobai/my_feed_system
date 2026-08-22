---
scenarios:
  - name: reviewer-opens-ops
    description: A signed-in reviewer opens operations from the administration workbench.
    expected: The operations surface shows a monitor tab and a logs tab, and a log search either lists lines or says there are none.
    tags:
      - frontend-e2e
      - desktop
  - name: ops-monitor-shows-product-event-stats
    description: A signed-in reviewer opens the operations monitor tab.
    expected: The embedded overview shows product-event statistics grouped by event name, not only a raw JSON stream of product_event lines.
    tags:
      - frontend-e2e
      - desktop
  - name: ordinary-account-hides-ops
    description: A visitor signed in without reviewer access looks at the account hub and the operations route.
    expected: No operations entry appears on the account hub. Opening `/ops` or `/admin/ops` returns the visitor to the account hub.
    tags:
      - frontend-e2e
      - desktop
