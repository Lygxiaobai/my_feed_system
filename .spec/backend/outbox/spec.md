---
title: outbox
status: active
code:
  - backend/internal/outbox/poller.go
related:
  - backend/internal/outbox/entity.go
  - backend/internal/outbox/repo.go
  - backend/internal/mq/publisher.go
  - backend/internal/mq/message.go
  - backend/internal/mq/topology.go
---
# outbox

## raw source
Business changes that require asynchronous work are published reliably through the outbox and message publisher boundary.

## expanded spec
The outbox separates the business transaction from message delivery. Pending messages can be retried, successful delivery is not duplicated beyond the consumer contract, and a temporary broker failure does not silently discard a committed business event.
