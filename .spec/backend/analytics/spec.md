---
title: analytics
status: active
code:
  - backend/internal/analytics/handler.go
related:
  - backend/internal/analytics/entity.go
  - backend/internal/http/router.go
---
# analytics

## raw source
The API accepts product-behavior events from the web client so usage can be inspected without mixing it into business writes.

## expanded spec
`POST /event/report` is a public write that may carry an optional login token. A request without a token is still accepted; a valid token attaches the account identity to every event in that batch. The endpoint is rate-limited by client IP so a noisy or looping client cannot flood the log pipeline. Failed limiter checks follow the same fail-open policy as other read-adjacent limits.

Each request carries a stable visitor identifier and a batch of events. The server accepts only the product event names the client is allowed to emit. Unknown names, an empty batch, a missing visitor identifier, or a batch larger than the documented cap are rejected with a caller error and write no event records. Property bags are sanitized: only primitive values are kept, sensitive keys are dropped, and oversized strings are truncated.

Accepted events are written as structured `product_event` log records. They are not persisted in the application database and they do not enter the outbox or worker path. The report request itself does not produce an access-log record; the event records are the source of truth for later retrieval in the log store.
