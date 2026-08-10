---
title: message-consumers
status: active
code:
  - backend/internal/mq/consumer.go
related:
  - backend/internal/mq/connection.go
  - backend/internal/mq/topology.go
  - backend/internal/mq/processed_message.go
  - backend/internal/mq/dead_letter_consumer.go
  - backend/internal/mq/dead_letter_message.go
  - backend/internal/worker/common.go
  - backend/internal/worker/comment_worker.go
  - backend/internal/worker/like_worker.go
  - backend/internal/worker/popularity_worker.go
  - backend/internal/worker/social_worker.go
  - backend/internal/worker/timeline_worker.go
  - backend/internal/feed/invalidation_consumer.go
  - backend/internal/video/invalidation_consumer.go
---
# message-consumers

## raw source
The Worker consumes RabbitMQ events and applies asynchronous media, comment, like, social, popularity, timeline, and cache-invalidation work safely.

## expanded spec
Consumers acknowledge messages only after the required effect is durable. Processed-message tracking makes redelivery safe, media tasks expose explicit processing/ready/failed states, and failed messages follow the dead-letter policy instead of disappearing. Worker behavior must remain independently runnable from the API process.
