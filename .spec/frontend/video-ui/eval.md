---
scenarios:
  - name: video-detail-navigation
    description: A user can open a video from a feed card and return without losing the route context.
    expected: The detail view shows the selected video or an explicit missing/error state, and back navigation returns to a usable feed.
    tags:
      - frontend-e2e
      - desktop
  - name: video-detail-resumes-unfinished
    description: A signed-in user reopens a detail page for a video they left unfinished.
    expected: The player seeks to the stored position after metadata is ready, and a completed or near-end video starts from the beginning.
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
  - name: video-upload-processing
    description: A user selects one video and starts publishing while the server transcodes it asynchronously.
    expected: Upload reports real byte-level progress, processing and publishing are shown as distinct in-progress states in user terms, polling covers only the account-owned task, publishing happens only after ready URLs exist, and a processing failure is shown as recoverable.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-rejects-invalid-file
    description: A user selects a non-video file or a video above the server size limit.
    expected: The file is rejected at selection time with the reason shown, no upload request is issued, and the publish action stays unavailable until an acceptable file is chosen.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-cancel
    description: A user cancels while the file is uploading or while the server is still processing it.
    expected: The in-flight request and any polling stop immediately, the form returns to an editable idle state, and cancellation is not reported as a failure.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-publish-has-no-cover-control
    description: A user completes the publish workflow end to end.
    expected: No cover is selected, uploaded, previewed, or confirmed at any point, and the published video shows the server-generated first-frame poster.
    tags:
      - frontend-e2e
      - desktop
      - mobile
