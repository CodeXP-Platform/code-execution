package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EventPublisher struct {
	conn *amqp.Connection
	cfg  config.MessagingConfig
}

func NewEventPublisher(conn *amqp.Connection, cfg config.MessagingConfig) *EventPublisher {
	return &EventPublisher{conn: conn, cfg: cfg}
}

func (p *EventPublisher) PublishExecutionStarted(ctx context.Context, event application.ExecutionStartedEvent) error {
	return p.publish(ctx, p.cfg.StartedRoutingKey, event)
}

func (p *EventPublisher) PublishExecutionCompleted(ctx context.Context, event application.ExecutionCompletedEvent) error {
	return p.publish(ctx, p.cfg.CompletedRoutingKey, event)
}

func (p *EventPublisher) publish(ctx context.Context, routingKey string, payload any) error {
	if p.conn == nil || p.conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is not available")
	}

	channel, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel failed: %w", err)
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare(
		p.cfg.ExecutionExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare execution exchange failed: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload failed: %w", err)
	}

	if err := channel.PublishWithContext(
		ctx,
		p.cfg.ExecutionExchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	); err != nil {
		return fmt.Errorf("publish event failed: %w", err)
	}

	return nil
}

var _ application.EventPublisher = (*EventPublisher)(nil)
