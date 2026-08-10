---
scenarios:
  - name: video-interactions
    description: A user can like, comment, follow, and unfollow from the supported video surfaces.
    expected: Each successful action updates the visible state and each failure is shown without a false success state.
    tags:
      - frontend-e2e
      - desktop
  - name: interaction-refresh
    description: A successful interaction remains consistent after navigating away and returning to the video.
    expected: The returned view reflects the server state rather than stale local action state.
    tags:
      - frontend-e2e
      - desktop
  - name: interaction-race-protection
    description: A user rapidly changes video context while comments or a like mutation is still settling.
    expected: A stale comment response is discarded, the like action remains pending until the transactional API response or failure, and the final visible state is not replaced by an older request.
    tags:
      - frontend-e2e
      - desktop
  - name: playback-surface-has-no-instructional-overlay
    description: A user views a feed video and a video detail page, then toggles mute.
    expected: No gesture or keyboard-shortcut hints are drawn over the video, mute produces no transient message because the control's own label shows the state, and the underlying gestures and shortcuts still work.
    tags:
      - frontend-e2e
      - desktop
      - mobile
