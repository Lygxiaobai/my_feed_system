---
title: admin
status: active
code:
  - backend/internal/admin/service.go
related:
  - backend/internal/admin/handler.go
  - backend/internal/admin/entity.go
  - backend/internal/report/service.go
  - backend/internal/video/service.go
  - backend/internal/account/service.go
  - backend/internal/http/router.go
---
# admin

## raw source
Reviewer accounts can open an administration surface that looks up accounts and videos regardless of public visibility, takes videos down with a written reason, reads the outstanding report queue, and opens the read-only operations console.

## expanded spec
Administration is a composition of existing reviewer powers, not a second permission system. The reviewer set is the same whitelist that already governs report handling, optional human review, and operations reads. An account that is merely bound to a test-domain email is not an administrator and cannot read logs or metrics. The API never returns the whitelist itself.

Access is answered to any signed-in caller so the client can hide the entry. Every other administration endpoint refuses a non-reviewer. Unauthenticated callers are refused as they are on any other protected route.

Looking up a video for a reviewer returns the persisted record even when it is pending, reviewing, or rejected, together with how many notices are still outstanding. That lookup must not use the public detail path: the public path answers “not found” for unapproved content so it cannot become a probe, and it must not write an unapproved record into the public detail cache. Looking up an account accepts exactly one of identifier, username, or email. The response may include the bound email and the author's works in every review state; it never includes a password, token, or credential.

A reviewer may take a video down without waiting for a report. The request must carry a non-empty reason. Removal happens first: the video enters the same rejected state that a failed review produces, the operator and reason are written to the durable trail under an administration source, and public caches of that video are invalidated. Outstanding notices for the same item are closed only after removal succeeds, so a failed removal cannot hide the case from the queue. Notices already closed are not rewritten.

The report queue and per-item decisions remain owned by `report/spec.md`. Administration does not duplicate those contracts; it only consumes them.

## change rules
Changing who may administer requires checking the reviewer whitelist used by `report/spec.md` and, when review is enabled, `video/spec.md`. Changing removal or visibility requires checking `video/spec.md` and `report/spec.md`, because administration shares that state machine and trail. Adding a new administration action that mutates money, credentials, or account existence requires a dedicated spec first.
