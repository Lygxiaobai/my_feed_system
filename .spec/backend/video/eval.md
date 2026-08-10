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
  - name: published-content-is-not-public-until-approved
    description: A user publishes a video and immediately checks every public listing.
    expected: The publish call succeeds, the video appears in the author's own view marked as awaiting review, and it appears in no public listing until a review approves it.
    tags:
      - backend-api
  - name: disallowed-text-is-rejected-including-evaded-spellings
    description: A user publishes titles containing a disallowed term, once plainly and once with inserted spacing, punctuation, and full-width characters.
    expected: Both are rejected, neither reaches any public listing, and the author is told only that the content did not pass, with no indication of what matched.
    tags:
      - backend-api
      - worker
  - name: undecidable-content-goes-to-a-human
    description: Content that the machine cannot decide is published, and separately the moderation path fails outright.
    expected: Both end up awaiting human review rather than approved, so a failure in the review path never becomes a publication channel.
    tags:
      - backend-api
      - worker
  - name: approval-publishes-through-one-path
    description: One video is approved by machine and another by a human reviewer.
    expected: Both become visible in the public listings and both count toward popularity, so neither decision route can expose content the other would not.
    tags:
      - backend-api
      - worker
  - name: unapproved-content-is-indistinguishable-from-missing
    description: A non-author requests an unapproved video directly and through every listing that can surface identifiers from a derived index.
    expected: Every route answers as though the video does not exist, and no derived index causes it to surface.
    tags:
      - backend-api
  - name: review-queue-strands-nothing
    description: A video's review event never reaches the worker.
    expected: The video still appears in the human review queue rather than remaining invisible to both the public and the reviewers.
    tags:
      - backend-api
  - name: review-actions-are-restricted-and-recorded
    description: A non-reviewer attempts a review action, then a reviewer decides the same item twice.
    expected: The non-reviewer is refused, the second decision is rejected as already handled, and every state change is recorded with its cause and operator.
    tags:
      - backend-api
  - name: pre-existing-content-survives-the-rollout
    description: The review feature is introduced into a system that already contains published videos.
    expected: Existing content stays publicly visible instead of disappearing behind a default awaiting-review state.
    tags:
      - backend-api
