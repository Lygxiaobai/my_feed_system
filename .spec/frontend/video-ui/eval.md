---
scenarios:
  - name: video-detail-navigation
    description: A user can open a video from a feed card and return without losing the route context.
    expected: The detail view shows the selected video or an explicit missing/error state, and back navigation returns to a usable feed.
    tags:
      - frontend-e2e
      - desktop
