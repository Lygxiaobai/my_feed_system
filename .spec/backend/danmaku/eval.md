---
scenarios:
  - name: danmaku-round-trip
    description: A signed-in user sends a short danmaku on a visible video, then lists that video's danmaku.
    expected: The sent item is in the list at the requested playback offset, and earlier offsets appear first.
    tags:
      - backend-api
  - name: danmaku-rejects-invalid-text-and-offset
    description: A signed-in user sends empty text, text longer than the cap, or a negative playback offset.
    expected: Each request is rejected as a caller error and no item is stored.
    tags:
      - backend-api
  - name: danmaku-hidden-video-looks-missing
    description: A viewer lists or sends danmaku for a video they cannot see.
    expected: Both answers are the same not-found outcome and reveal nothing about whether the video exists.
    tags:
      - backend-api
