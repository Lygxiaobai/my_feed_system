---
scenarios:
  - name: video-detail-navigation
    description: A user can open a video from a feed card and return without losing the route context.
    expected: The detail view shows the selected video or an explicit missing/error state, and back navigation returns to a usable feed.
    tags:
      - frontend-e2e
      - desktop
  - name: video-playback-lifecycle
    description: The active feed or detail video can autoplay muted, pause when inactive, and expose loading, buffering, failure, and retry states.
    expected: Only the active feed video plays, immediate neighbors may preload, failed playback is visible and recoverable, and leaving the view releases playback resources.
    tags:
      - frontend-e2e
      - desktop
      - mobile
