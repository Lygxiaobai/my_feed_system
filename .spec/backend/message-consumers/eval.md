---
scenarios:
  - name: consumer-redelivery
    description: A Worker receives the same message more than once after a delivery interruption.
    expected: The durable business effect is applied once according to the consumer contract and the message is acknowledged only after the required effect succeeds.
    tags:
      - backend-api
  - name: consumer-dead-letter
    description: A message that cannot be processed successfully reaches the configured dead-letter path.
    expected: The failed message is not silently lost and its failure context remains available for inspection.
    tags:
      - backend-api
  - name: media-transcode-consumer
    description: The Worker receives a media.transcode.requested event for an uploaded video.
    expected: The task is acknowledged only after it reaches ready or failed durably; redelivery does not publish a second media result.
    tags:
      - worker
      - media
