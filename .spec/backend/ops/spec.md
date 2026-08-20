---
title: ops
status: active
code:
  - backend/internal/ops/service.go
related:
  - backend/internal/ops/handler.go
  - backend/internal/account/service.go
  - backend/internal/http/router.go
---
# ops

## raw source
Accounts bound to a digits-only address on the configured test email domain can read operations data: Loki logs, a few Prometheus numbers, and a Grafana dashboard behind the site login.

## expanded spec
Ops access is not a second login. It follows from the existing test-email identity. An account without that identity, or a request without a valid session, cannot read logs or open the Grafana proxy.

The API does not return Grafana credentials. Log queries are bounded in time, size, and shape, and a failed observability lookup does not expose Loki or Prometheus error text. The Grafana gate is omitted from access logs because each dashboard load would otherwise emit a burst of identical records.
