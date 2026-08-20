---
title: runtime
status: active
code:
  - backend/internal/http/router.go
related:
  - backend/internal/config/loadconfig.go
  - backend/internal/db/db.go
  - backend/internal/db/gormlog.go
  - backend/internal/logging/logging.go
  - backend/internal/response/code.go
  - backend/internal/response/response.go
  - backend/internal/middleware/accesslog/accesslog.go
  - backend/internal/middleware/requestid/requestid.go
  - backend/internal/mq/connection.go
  - backend/internal/observability/metrics.go
  - backend/internal/observability/pprof.go
---
# runtime

## raw source
The backend exposes one HTTP API surface with public and authenticated routes, plus one separately runnable asynchronous Worker. Both use shared configuration and observability conventions while keeping their process responsibilities separate.

## expanded spec
Routes, middleware, database initialization, broker connections, metrics, and pprof are assembled without making the Worker depend on the API process. Configuration errors fail loudly at startup. Runtime changes must preserve the public API contract and the separation between synchronous requests and asynchronous work.

The response envelope and the failure-code vocabulary have a single owner; handlers select a code and a user-facing message and never assemble a response body themselves. The writing path accepts the underlying error only as a logging input, so no call site is able to place internal error text in a response.

Logging has one owner as well: severity thresholds, output shape, redaction, and request-identifier propagation are configured once for both processes and are not re-implemented per package. Library loggers that would otherwise write in their own format are adapted into it rather than left to write directly. The framework's own request logger is replaced so that request outcome, latency, and route become fields rather than a preformatted line. Severity level and output shape are configurable without a rebuild, and the deployment bounds retained log volume so that an unattended process cannot exhaust disk. Product-event report requests are omitted from access logs because each accepted event is written as its own product_event record.

Configuration carries no credential values. Every secret is a placeholder resolved from the environment at load time, so the repository can be published without exposing anything; a value that is merely obscured in the file does not satisfy this, because the means of recovering it must then live somewhere too. Placeholders distinguish required from defaulted: a missing required value aborts startup with every missing name reported at once, and is never resolved to an empty string, because a silently empty signing key accepts forged credentials while the service appears healthy. Values that are safe to publish may carry defaults so a developer can start the system by supplying only the secrets. Resolution order is process environment first, then an untracked local file, then the declared default. Startup additionally rejects a signing key that is absent or too short to resist offline attack.
