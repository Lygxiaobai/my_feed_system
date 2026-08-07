---
scenarios:
  - name: publish-video
    description: An authenticated user can upload the required media and publish one video.
    expected: The publish response identifies the video and its media URLs, and the operation is not duplicated by a repeated idempotency key.
    tags:
      - backend-api
  - name: video-detail-cache-fallback
    description: A video detail remains readable when the detail cache is unavailable.
    expected: The API returns the persisted video detail through the database fallback.
    tags:
      - backend-api
