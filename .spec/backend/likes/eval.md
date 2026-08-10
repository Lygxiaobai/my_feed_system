---
scenarios:
  - name: like-state-idempotency
    description: A user can like, query, unlike, and query a video without duplicate relationship effects.
    expected: The final like state is false and the visible count reflects one like transition rather than repeated increments.
    tags:
      - backend-api
  - name: like-write-consistency
    description: A successful like or unlike request is followed immediately by a relationship query and video detail read.
    expected: The relationship and likes_count already reflect the mutation before any MQ consumer runs; duplicate or repeated transitions do not double-count.
    tags:
      - backend-api
      - mysql
