---
scenarios:
  - name: outbox-retry
    description: A committed business change remains deliverable when the message broker is temporarily unavailable.
    expected: The pending outbox record is retained, a later poll retries publication, and the event is not silently discarded.
    tags:
      - backend-api
