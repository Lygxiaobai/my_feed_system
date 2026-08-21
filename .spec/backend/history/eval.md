---
scenarios:
  - name: history-upsert-skips-shallow-and-saves-midway
    description: A signed-in user reports 1s of a long video, then 8s of the same video.
    expected: The shallow report is accepted but not stored. The midway report is stored as unfinished and returns a non-zero resume offset.
    tags:
      - backend-api
  - name: history-completed-starts-over
    description: A signed-in user reports a position within two seconds of the end, then reports position 0 after a loop.
    expected: The row is completed with a zero stored position. The zero report does not turn it back into unfinished.
    tags:
      - backend-api
  - name: history-hidden-video-looks-missing
    description: A viewer upserts or reads progress for a video they cannot see.
    expected: Upsert answers with the same not-found outcome as video detail. Progress omits that identifier and does not reveal that the video exists.
    tags:
      - backend-api
  - name: history-list-pages-visible-videos
    description: A signed-in user lists unfinished history after watching two visible videos.
    expected: Items arrive newest first, a cursor returns the remaining page, and an invalid status is rejected.
    tags:
      - backend-api
