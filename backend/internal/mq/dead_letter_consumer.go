package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeadLetterConsumer struct {
	conn        *amqp.Connection
	db          *gorm.DB
	queue       string
	consumerTag string
	prefetch    int
}

func NewDeadLetterConsumer(conn *amqp.Connection, db *gorm.DB, queue string, consumerTag string, prefetchCount int) *DeadLetterConsumer {
	if prefetchCount <= 0 {
		prefetchCount = 5
	}
	return &DeadLetterConsumer{
		conn:        conn,
		db:          db,
		queue:       queue,
		consumerTag: consumerTag,
		prefetch:    prefetchCount,
	}
}

func (c *DeadLetterConsumer) Run(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("rabbitmq connection is nil")
	}
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open dlq consumer channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		return fmt.Errorf("set dlq qos: %w", err)
	}

	msgs, err := ch.Consume(
		c.queue,
		c.consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume dlq %s: %w", c.queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("dlq consumer channel closed: %s", c.queue)
			}

			if err := c.store(d); err != nil {
				slog.ErrorContext(ctx, "store dead letter failed, keeping message unacked",
					slog.String("queue", c.queue), slog.String("error", err.Error()))
				if nackErr := d.Nack(false, true); nackErr != nil {
					slog.ErrorContext(ctx, "nack requeue failed",
						slog.String("queue", c.queue), slog.String("error", nackErr.Error()))
				}
				continue
			}

			if err := d.Ack(false); err != nil {
				slog.ErrorContext(ctx, "ack failed",
					slog.String("queue", c.queue), slog.String("error", err.Error()))
			}
		}
	}
}

func (c *DeadLetterConsumer) store(d amqp.Delivery) error {
	var event Envelope
	if err := json.Unmarshal(d.Body, &event); err != nil {
		row := DeadLetterMessage{
			EventID:        d.MessageId,
			EventType:      d.Type,
			SourceQueue:    "unknown",
			DLQ:            c.queue,
			BodySHA256:     bodySHA256(d.Body),
			Exchange:       d.Exchange,
			RoutingKey:     d.RoutingKey,
			MessageID:      d.MessageId,
			DeathReason:    "invalid-json",
			Headers:        "{}",
			Body:           string(d.Body),
			HandledStatus:  "stored",
			HandledComment: fmt.Sprintf("unmarshal envelope failed: %v", err),
		}
		return c.insert(row)
	}

	row := NewDeadLetterMessage(c.queue, d, event)
	return c.insert(row)
}

func (c *DeadLetterConsumer) insert(row DeadLetterMessage) error {
	return c.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dlq"}, {Name: "body_sha256"}},
		DoNothing: true,
	}).Create(&row).Error
}
