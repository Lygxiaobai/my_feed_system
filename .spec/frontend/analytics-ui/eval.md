---
scenarios:
  - name: page-view-is-recorded-on-navigation
    description: A user opens the site and then moves from the recommendation feed to the hot ranking.
    expected: Two page_view events are reported, one for each destination, and the UI navigation is unchanged.
    tags:
      - frontend-e2e
      - desktop
  - name: successful-like-is-recorded
    description: A logged-in user likes a video from the feed.
    expected: A video_like event is reported after the like request succeeds; a failed like does not record the event and still shows the previous state.
    tags:
      - frontend-e2e
      - desktop
  - name: watch-duration-is-recorded-on-leave
    description: A user plays a feed video and then scrolls to the next item.
    expected: The first video emits video_play when playback starts and video_watch when it is left, carrying the elapsed watch time when available.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: tracking-failure-is-invisible
    description: The report endpoint is unavailable while the user publishes a comment.
    expected: The comment still appears in the open drawer and the user is not shown a tracking error.
    tags:
      - frontend-e2e
      - desktop
