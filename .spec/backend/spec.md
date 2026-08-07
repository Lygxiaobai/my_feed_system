---
title: backend
status: active
code:
  - backend/cmd/main.go
  - backend/cmd/worker/main.go
related:
  - backend/internal/http/router.go
  - backend/internal/config/loadconfig.go
  - backend/internal/mq/topology.go
  - backend/internal/observability/metrics.go
desc: Go API and asynchronous Worker architecture boundary.
---
# backend

## raw source
The backend has two runnable processes: an HTTP API and an asynchronous Worker. They share configuration and infrastructure packages, while their process responsibilities remain separate.

## expanded spec
`backend/cmd/main.go` owns API bootstrap and lifecycle. It loads configuration, initializes database and optional cache or broker dependencies, assembles the HTTP router, starts API-side outbox and cache-invalidation work, and serves the public API.

`backend/cmd/worker/main.go` owns asynchronous processing. It connects to the broker, starts the like, comment, social, popularity, timeline, and dead-letter consumers, and shuts them down with the process context. It does not replace the HTTP API process.

`backend/internal/http/router.go` is the API composition boundary. Routes, authentication middleware, rate limits, and handler wiring belong there; capability behavior belongs to the child backend specs. Configuration loading and shared database, broker, cache, and observability conventions must remain usable by both entrypoints.

The child nodes own the detailed account, feed, video, interaction, social, outbox, message-consumer, and runtime contracts. This parent spec owns only the process boundary and the rules that connect those nodes.

## change rules
Changing an API route, middleware, or API startup dependency requires checking `runtime/spec.md` and the affected capability spec. Changing a queue, event, consumer, or worker lifecycle requires checking `message-consumers/spec.md`, `outbox/spec.md`, and the affected capability spec. Changing shared configuration or an entrypoint requires checking both API and Worker startup paths.
