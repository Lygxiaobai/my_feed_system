---
title: danmaku
status: active
code:
  - backend/internal/danmaku/service.go
related:
  - backend/internal/danmaku/handler.go
  - backend/internal/danmaku/repo.go
  - backend/internal/danmaku/entity.go
  - backend/internal/danmaku/access.go
  - backend/internal/video/service.go
---
# danmaku

## raw source
Signed-in viewers can send a short timed overlay comment on a video they can see. Anyone who can see that video can list those comments in playback order.

## expanded spec
Danmaku is a playback overlay, not a discussion tree. Each item stores the text and the playback offset at which it was sent, so later viewers see it at the same moment in the video. It does not reply, does not change comment counts, and does not enter the popularity or outbox path.

Listing and sending both go through the same visibility rule as opening the video. A viewer who cannot see the video cannot distinguish a missing video from a hidden one: both answers are the same not-found outcome. Optional login on the list path exists only so an author can read danmaku on their own unapproved video.

Sending requires a signed-in account. The text is trimmed, must not be empty, and is capped at a short length so the overlay remains readable. The playback offset cannot be negative or larger than a day of media, because those values are not a real playback position. An accepted send is written before the response returns, so the sender can immediately retrieve the same item.

A list is bounded and ordered by playback offset, then identity, so a client can replay the overlay without paging during a short video.

## change rules
Any change that lets danmaku reveal a video the viewer cannot open contradicts this contract and must be refused. Adding replies, counters, or asynchronous persistence is a new capability and requires updating this spec first.
