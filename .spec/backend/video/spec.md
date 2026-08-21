---
title: video
status: active
code:
  - backend/internal/video/service.go
related:
  - backend/internal/video/handler.go
  - backend/internal/video/repo.go
  - backend/internal/video/entity.go
  - backend/internal/video/detail_cache.go
  - backend/internal/video/detail_cache_payload.go
  - backend/internal/video/local_detail_cache.go
  - backend/internal/video/media_validator.go
  - backend/internal/video/invalidation_consumer.go
  - backend/internal/video/sharecode.go
  - backend/internal/media/entity.go
  - backend/internal/media/repo.go
  - backend/internal/media/service.go
  - backend/internal/media/processor.go
  - backend/internal/worker/media_worker.go
  - backend/internal/audit/moderator.go
  - backend/internal/audit/entity.go
  - backend/internal/audit/service.go
  - backend/internal/video/audit_store.go
  - backend/internal/worker/audit_worker.go
---
# video

## raw source
The video subsystem validates uploads, publishes videos, serves details, exposes author and liked-video views, and issues shareable codes that resolve back to a video. Content review is optional and off by default; when enabled, published content is reviewed before it becomes publicly discoverable.

## expanded spec
Video publishing is authenticated and idempotent. Upload and publish are rate-limited by client IP and by account so a single session cannot flood the media worker; public detail and share reads share a separate IP ceiling. A video upload creates an account-owned `processing` media task and returns no playable URL until the Worker has produced a standard MP4 and JPEG poster. Tasks finish as `ready` with `/static/videos/*.mp4` and `/static/covers/*.jpg` URLs or as `failed` with a bounded error message.

The Worker uses ffmpeg to normalize video codec/container and MIME-by-extension, enables MP4 fast start, and generates the poster. Raw source files remain private to the shared upload volume and are not exposed by the static resource surface. Media paths returned by the API remain usable through the static resource surface. Detail reads may use cache but must preserve the persisted video's visible fields. Publishing accepts only ready, playable media.

Review is gated by `audit.enabled` and defaults to off. When review is disabled, publishing writes an approved status and immediately enqueues the same public-release side effects (global timeline and popularity) that approval would have produced. The machine-review consumer and human-review HTTP surface stay unregistered.

When review is enabled, publishing does not make content public. A newly published video is awaiting review and is visible only to its author until a review decision approves it. Review is decided by machine first and escalates to a human whenever the machine cannot decide or the moderation path itself fails; a moderation failure never resolves to approval, because an outage in the review path must not become a publication channel. A review outcome and its audit trail are recorded together, so no state change exists without an explanation of who or what caused it, and the trail outlives the log retention window.

Approval — or publish itself when review is disabled — is the single point where content becomes public: the side effects that expose content — entering the global timeline, counting toward popularity, and scheduling a title-and-description embedding for recommendation — happen there and only there, for both machine and human decisions. A failed embedding must not block publication; the video remains public and can still appear in non-interest recommendation queues. Consequently no read path may treat presence in a derived index as evidence of approval; every public query filters on review state, including the one that resolves cached or index-supplied identifiers back into videos. A viewer who is not the author cannot distinguish an unapproved video from one that does not exist. Content awaiting review remains reachable by a human reviewer even if its machine review never arrived, so nothing can be silently stranded outside both the public surface and the review queue.

A rejection tells the author only that the content did not pass. The matched rule, category, or any other decision detail stays internal, because disclosing it lets an author probe the boundary by resubmitting variations.

The review capability is an interface with one local implementation; substituting or chaining a different provider must not require changes to the state machine, the audit trail, or the read-path filtering.

A video can be addressed by a share code as well as by its identifier. The code is derived from the video itself and stores nothing, so a code never expires, never drifts from the content it names, and needs no cleanup. Codes are fixed width and carry a check character, so a code that was truncated or mistyped is reported as invalid rather than silently resolving to a different video — resolving to the wrong video is worse than refusing, because the viewer has no way to notice.

The code is an addressing scheme, not a credential. It grants nothing: resolving a code applies exactly the same visibility rules as requesting the video directly, so a code for content that is unapproved or removed resolves as though the content does not exist, and issuing a code for a video the requester cannot see is likewise impossible. Consequently the encoding may obscure the identifier's sequence but must never be relied on to conceal it.

Both directions of the mapping live on the server, and resolution accepts the surrounding text a user pasted rather than requiring a pre-extracted code. Recognition is deliberately conservative: text is only treated as carrying a code when it is unambiguously one, because a scheme that guesses will occasionally send a viewer to an unrelated video. Text submitted for resolution is bounded, and every failure to recognize, validate, or resolve a code is reported to the caller as the same outcome, so a caller holding arbitrary codes learns nothing about which ones name real content.
