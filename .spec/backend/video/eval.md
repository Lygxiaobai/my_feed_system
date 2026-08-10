---
scenarios:
  - name: publish-video
    description: An authenticated user uploads a video, waits for media processing, and publishes one video.
    expected: The upload task reaches ready with a playable MP4 and generated poster before publish; the publish response identifies the video and the operation is not duplicated by a repeated idempotency key.
    tags:
      - backend-api
  - name: media-task-failure
    description: An uploaded file that ffmpeg cannot decode is processed by the Worker.
    expected: The task reaches failed with a bounded error message, no raw source path is returned, and the file cannot be published as a playable video.
    tags:
      - backend-api
      - worker
  - name: video-detail-cache-fallback
    description: A video detail remains readable when the detail cache is unavailable.
    expected: The API returns the persisted video detail through the database fallback.
    tags:
      - backend-api
