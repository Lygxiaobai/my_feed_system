---
scenarios:
  - name: like-state-idempotency
    description: A user can like, query, unlike, and query a video without duplicate relationship effects.
    expected: The final like state is false and the visible count reflects one like transition rather than repeated increments.
    tags:
      - backend-api
