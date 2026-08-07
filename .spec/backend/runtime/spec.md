---
title: runtime
status: active
code:
  - backend/internal/http/router.go
related:
  - backend/internal/config/loadconfig.go
  - backend/internal/db/db.go
  - backend/internal/mq/connection.go
  - backend/internal/observability/metrics.go
  - backend/internal/observability/pprof.go
---
# runtime

## raw source
The backend exposes one HTTP API surface with public and authenticated routes, plus one separately runnable asynchronous Worker. Both use shared configuration and observability conventions while keeping their process responsibilities separate.

## expanded spec
Routes, middleware, database initialization, broker connections, metrics, and pprof are assembled without making the Worker depend on the API process. Configuration errors fail loudly at startup. Runtime changes must preserve the public API contract and the separation between synchronous requests and asynchronous work.
