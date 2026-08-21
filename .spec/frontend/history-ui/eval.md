---
scenarios:
  - name: account-history-tabs
    description: A signed-in user opens browsing history and switches between unfinished and completed.
    expected: The account hub exposes 浏览历史 beside the wallet actions and as a metric. Each tab shows only that status, unfinished cards show a progress bar, completed cards show 已看完, and opening a card goes to the video detail.
    tags:
      - frontend-e2e
      - desktop
  - name: detail-resumes-unfinished
    description: A signed-in user leaves a video midway, then opens that video from history.
    expected: After metadata is ready the player seeks to the stored unfinished position and does not start from the last two seconds.
    tags:
      - frontend-e2e
      - desktop
  - name: feed-does-not-auto-seek
    description: A signed-in user has unfinished progress on a video and then swipes onto that video in the feed.
    expected: The feed item starts from the beginning. Progress is still recorded when the user pauses or leaves.
    tags:
      - frontend-e2e
      - desktop
      - mobile
