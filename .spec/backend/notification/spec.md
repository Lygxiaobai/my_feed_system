---
title: notification
status: active
code:
  - backend/internal/notification/service.go
related:
  - backend/internal/notification/entity.go
  - backend/internal/notification/writer.go
  - backend/internal/notification/mentions.go
  - backend/internal/notification/handler.go
  - backend/internal/notification/repo.go
  - backend/internal/like/service.go
  - backend/internal/comment/service.go
  - backend/internal/social/service.go
  - backend/internal/wallet/service.go
  - backend/internal/worker/comment_worker.go
  - backend/internal/worker/social_worker.go
---
# notification

## raw source
A signed-in user receives inbox notices when someone follows them, likes their video, comments or replies, mentions them with `@username`, or tips their video.

## expanded spec
Notifications are a projection of already-committed social facts, not a second write path. They are written in the same transaction that makes the like, comment, follow, or tip durable, so a successful mutation is visible both as the original relationship and as an inbox row after that transaction commits. The recipient is never the actor: liking, commenting on, mentioning, following, or tipping yourself produces no notice.

Likes on one video collapse into a single row for the author. A later like updates the latest actors, raises the participant count, unhides the row if it was cleared, and marks it unread again. An unlike removes that actor; when nobody remains the row is hidden rather than left as an empty like. Follow, comment, reply, mention, and tip rows stay one-to-one with the triggering fact. Unfollow does not retract a follow notice. Deleting a comment hides the comment, reply, and mention rows for that comment and for its reply subtree.

A root comment notifies the video author as a comment. A reply notifies the person being replied to as a reply. `@username` tokens in the comment body notify those accounts as mentions, except anyone already receiving the comment or reply for that same body, so one sentence does not produce two bells for the same person. Unknown names are ignored. Mentions are capped so a single comment cannot fan out without bound.

The inbox is private. Listing, unread counts, and mark-read only see the caller's rows. Hidden rows do not appear and do not count as unread. The list can be filtered by follow, like, mention, tip, or the comment group that includes both comments and replies. Each row carries the actors that should be shown, a short action sentence, optional comment text, a video cover when the target still exists, the spend amount for a tip, and whether the recipient already follows the primary actor.

## change rules
Changing which actions write a notice, who the recipient is, or how likes aggregate requires checking the like, comment, social, and wallet specs, because those transactions are the source of the facts. Changing list filters or the unread contract requires checking `frontend/notification-ui`.
