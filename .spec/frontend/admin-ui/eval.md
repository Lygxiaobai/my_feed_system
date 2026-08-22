---
scenarios:
  - name: reviewer-opens-admin
    description: A signed-in reviewer opens administration from the account hub.
    expected: The workbench shows overview, report queue, video lookup, account lookup, and operations. The consumer chrome is absent.
    tags:
      - frontend-e2e
      - desktop
  - name: ordinary-account-hides-admin
    description: A visitor signed in without reviewer access looks at the account hub and the administration route.
    expected: The administration entry is absent, and opening the route returns the visitor to the account hub.
    tags:
      - frontend-e2e
      - desktop
  - name: report-queue-decides-an-item
    description: A reviewer opens the report queue that contains one video with several notices, then dismisses or removes it.
    expected: The queue shows one row for that video with its reason totals. After a confirmed decision the item leaves the queue. A removal without a reason does not proceed.
    tags:
      - frontend-e2e
      - desktop
  - name: video-lookup-shows-hidden-content
    description: A reviewer looks up a rejected video by identifier.
    expected: The workbench shows the title, review state, and playback. The same identifier on the public detail route still appears missing to a non-author.
    tags:
      - frontend-e2e
      - desktop
