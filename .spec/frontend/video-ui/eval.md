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
  - name: video-playback-time-and-progress
    description: A user watches an active feed or detail video and uses the progress bar.
    expected: The player shows current time and total duration as mm:ss / mm:ss. The bar follows playback, and tapping or dragging it seeks to that offset so overlays such as danmaku stay aligned.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-processing
    description: A signed-in user selects or drops one video. Upload starts immediately while they edit the title and description, then they publish after the server transcodes it.
    expected: Upload starts on accept without waiting for the publish click, reports real byte-level progress that stays below 100% until the server accepts the file, shows a confirming state after bytes leave the browser, keeps title and description editable during upload and processing, shows processing and publishing as distinct in-progress states in user terms, polls only the account-owned task, publishes only after ready URLs exist, and shows a processing failure as recoverable without requiring a new file.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-progress-waits-for-ack
    description: A signed-in user uploads a video and watches the progress bar before the server responds.
    expected: The bar follows sent bytes but does not show 100% before the upload request completes. After the browser finishes sending, the UI shows a confirming state in user terms, then switches to processing only after the server accepts the file.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-rejects-invalid-file
    description: A user selects or drops a non-video file or a video above the server size limit.
    expected: The file is rejected at selection or drop time with the reason shown, no upload request is issued, and the publish action stays unavailable until an acceptable file is chosen.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-upload-cancel
    description: A user cancels while the file is uploading or while the server is still processing it.
    expected: The in-flight request and any polling stop immediately, the form stays editable with the file still selected, and cancellation is not reported as a failure.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: video-publish-waits-for-ready
    description: A user clicks publish before upload or processing has finished.
    expected: The click does not start a second upload. The workflow waits for ready playable URLs, then publishes once using the title and description current at send time.
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
