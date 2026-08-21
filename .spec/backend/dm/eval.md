---
scenarios:
  - name: mutual-friends-unlimited
    description: Two accounts follow each other and exchange several text messages.
    expected: Every send succeeds, the recipient inbox shows the latest preview, and opening the thread lists the messages in time order.
    tags:
      - backend-api
  - name: stranger-one-message-quota
    description: An account that does not mutually follow the peer sends a first message and then a second.
    expected: The first send succeeds and the second is rejected with the quota code; the peer can still send their own single greeting.
    tags:
      - backend-api
  - name: one-way-follow-still-capped
    description: A user follows the peer but is not followed back, then tries to send two messages.
    expected: Only the first message is stored.
    tags:
      - backend-api
  - name: becoming-friends-unlocks-chat
    description: A stranger sends their one allowed message, then the pair become mutual followers and the same sender writes again.
    expected: The later send succeeds and remaining becomes unlimited.
    tags:
      - backend-api
  - name: rejects-self-empty-long-and-missing
    description: A signed-in user messages themself, an unknown account, empty text, or text longer than 500 characters.
    expected: Each request is a caller error and no extra message row is stored.
    tags:
      - backend-api
  - name: open-thread-marks-read
    description: The recipient has one unread message, opens the thread, then the sender reloads the thread.
    expected: The recipient unread count returns to zero and the sender sees the latest own message as read.
    tags:
      - backend-api
