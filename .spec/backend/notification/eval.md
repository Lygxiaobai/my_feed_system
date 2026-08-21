---
scenarios:
  - name: like-notices-aggregate-then-retract
    description: Two accounts like the same video, one repeats the like, then both unlike.
    expected: The author sees one like notice with two participants; the repeat does not raise the count; the last unlike hides the notice.
    tags:
      - backend-api
  - name: self-actions-are-silent
    description: An account likes, comments on, mentions, follows, and tips its own content.
    expected: The inbox stays empty.
    tags:
      - backend-api
  - name: comment-reply-and-mention-split
    description: A viewer comments on a video and @-mentions another account, then a third account replies and mentions both the commenter and a fourth account.
    expected: The author gets a comment, the commenter gets a reply, mentioned accounts get mentions, and nobody gets two notices for the same sentence. Deleting the root comment hides the whole set.
    tags:
      - backend-api
  - name: follow-and-tip-are-private
    description: One account follows and tips another, then both list and mark notices.
    expected: Only the recipient sees the rows and unread counts; marking one then all clears unread without exposing the inbox to the other account.
    tags:
      - backend-api
