---
scenarios:
  - name: follow-state
    description: A user can follow and unfollow another user and read the resulting relationship state.
    expected: Self-follow is rejected, repeated requests are idempotent, and the following list matches the final state.
    tags:
      - backend-api
