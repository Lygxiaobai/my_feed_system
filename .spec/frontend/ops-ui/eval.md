---
scenarios:
  - name: test-email-opens-ops
    description: A visitor signed in with a digits-only test-domain email opens operations from the account hub.
    expected: The operations surface shows a monitor tab and a logs tab, and a log search either lists lines or says there are none.
    tags:
      - frontend-e2e
      - desktop
  - name: ops-monitor-shows-product-event-stats
    description: A visitor signed in with a digits-only test-domain email opens the operations monitor tab.
    expected: The embedded overview shows product-event statistics grouped by event name, not only a raw JSON stream of product_event lines.
    tags:
      - frontend-e2e
      - desktop
  - name: ordinary-account-hides-ops
    description: A visitor signed in without a test-email identity looks at the account hub and the operations route.
    expected: The operations entry is absent, and opening the route returns the visitor to the account hub.
    tags:
      - frontend-e2e
      - desktop
