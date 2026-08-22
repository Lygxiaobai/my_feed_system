package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type HandlerFunc func(ctx context.Context, event Envelope) error

const defaultHandleTimeout = 10 * time.Second

type Consumer struct {
	conn          *amqp.Connection
	queue         string
	consumerTag   string
	prefetch      int
	handle        HandlerFunc
	handleTimeout time.Duration
}

func NewConsumer(conn *amqp.Connection, queue string, consumerTag string, prefetchCount int, handle HandlerFunc) *Consumer {
	if prefetchCount <= 0 {
		prefetchCount = 10
	}
	return &Consumer{
		conn:          conn,
		queue:         queue,
		consumerTag:   consumerTag,
		prefetch:      prefetchCount,
		handle:        handle,
		handleTimeout: defaultHandleTimeout,
	}
}

// SetHandleTimeout 覆盖单条消息的处理时限。转码这类慢活必须长于默认的 10 秒，
// 否则 CommandContext 会 SIGKILL 掉还在跑的 ffmpeg。
func (c *Consumer) SetHandleTimeout(d time.Duration) *Consumer {
	if d > 0 {
		c.handleTimeout = d
	}
	return c
}

func (c *Consumer) Run(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	msgs, err := ch.Consume(
		c.queue,
		c.consumerTag,
		false, // 关闭自动 ACK，只有业务处理成功后才确认。
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", c.queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("consumer channel closed: %s", c.queue)
			}

			if err := c.handleDelivery(ctx, d, c.handleTimeout); err != nil {
				// nack(requeue=false) 会按队列策略进入死信队列。
				slog.ErrorContext(ctx, "handle message failed, sending to dlq",
					slog.String("queue", c.queue), slog.String("error", err.Error()))
				if nackErr := d.Nack(false, false); nackErr != nil {
					slog.ErrorContext(ctx, "nack failed",
						slog.String("queue", c.queue), slog.String("error", nackErr.Error()))
				}
				continue
			}

			// 业务处理成功后再 ACK。
			if err := d.Ack(false); err != nil {
				slog.ErrorContext(ctx, "ack failed",
					slog.String("queue", c.queue), slog.String("error", err.Error()))
			}
		}
	}
}

func (c *Consumer) handleDelivery(parent context.Context, d amqp.Delivery, timeout time.Duration) error {
	return handleDelivery(parent, d, c.handle, timeout)
}

func ConsumeEphemeralFanout(ctx context.Context, conn *amqp.Connection, exchange string, consumerTag string, prefetchCount int, handle HandlerFunc) error {
	if conn == nil {
		return fmt.Errorf("rabbitmq connection is nil")
	}
	if handle == nil {
		return fmt.Errorf("consumer handler is nil")
	}
	if prefetchCount <= 0 {
		prefetchCount = 10
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	queue, err := ch.QueueDeclare(
		//临时队列
		"",
		false,
		true,
		true,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare ephemeral queue: %w", err)
	}

	if err := ch.QueueBind(queue.Name, "", exchange, false, nil); err != nil {
		return fmt.Errorf("bind ephemeral queue %s to exchange %s: %w", queue.Name, exchange, err)
	}

	msgs, err := ch.Consume(
		queue.Name,
		consumerTag,
		false,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", queue.Name, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("consumer channel closed: %s", queue.Name)
			}

			if err := handleDelivery(ctx, d, handle, defaultHandleTimeout); err != nil {
				slog.ErrorContext(ctx, "handle message failed, dropping fanout message",
					slog.String("exchange", exchange), slog.String("error", err.Error()))
				if nackErr := d.Nack(false, false); nackErr != nil {
					slog.ErrorContext(ctx, "nack failed",
						slog.String("exchange", exchange), slog.String("error", nackErr.Error()))
				}
				continue
			}

			if err := d.Ack(false); err != nil {
				slog.ErrorContext(ctx, "ack failed",
					slog.String("exchange", exchange), slog.String("error", err.Error()))
			}
		}
	}
}

func handleDelivery(parent context.Context, d amqp.Delivery, handle HandlerFunc, timeout time.Duration) error {
	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if timeout <= 0 {
		timeout = defaultHandleTimeout
	}

	handleCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	return handle(handleCtx, env)
}
