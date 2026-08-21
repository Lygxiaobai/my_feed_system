---
scenarios:
  - name: follow-state
    description: A user can follow and unfollow another user and read the resulting relationship state.
    expected: Self-follow is rejected, repeated requests are idempotent, and the following list matches the final state.
    tags:
      - backend-api
  - name: follower-count-consistency
    description: A follow is applied, retried, and then reversed, including an unfollow of a relationship that no longer exists.
    expected: The follower count increases once, never increases on a retry, returns to its previous value, and never becomes negative.
    tags:
      - worker
  - name: follow-refreshes-following-feed
    description: A user follows a new author and then opens the following feed.
    expected: The feed includes that author's existing videos rather than only videos published after the follow.
    tags:
      - backend-api
