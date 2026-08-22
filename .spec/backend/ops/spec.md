---
title: ops
status: active
code:
  - backend/internal/ops/service.go
related:
  - backend/internal/ops/handler.go
  - backend/internal/http/router.go
---
# ops

## raw source
Reviewer accounts can read operations data: Loki logs, a few Prometheus numbers, and a Grafana dashboard behind the site login. The dashboard includes product-event statistics from the log store, grouped by event name, in addition to API RED metrics.

## expanded spec
Ops access is not a second login and is not implied by a test-domain email. It follows from the same reviewer whitelist that governs administration. An account that is not a reviewer, or a request without a valid session, cannot read logs or open the Grafana proxy. The test-email identity that once opened this surface no longer does.

The API does not return Grafana credentials. Log queries are bounded in time, size, and shape, and a failed observability lookup does not expose Loki or Prometheus error text. The Grafana gate is omitted from access logs because each dashboard load would otherwise emit a burst of identical records. The overview dashboard aggregates product_event log records by event name so operators can see usage counts without reading raw JSON lines.
