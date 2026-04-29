package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"code-execution/internal/config"
	"code-execution/internal/execution/application"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RequestedConsumer struct {
	conn           *amqp.Connection
	cfg            config.MessagingConfig
	executeUseCase *application.ExecuteSolutionUseCase
}

func NewRequestedConsumer(
	conn *amqp.Connection,
	cfg config.MessagingConfig,
	executeUseCase *application.ExecuteSolutionUseCase,
) *RequestedConsumer {
	return &RequestedConsumer{conn: conn, cfg: cfg, executeUseCase: executeUseCase}
}

func (c *RequestedConsumer) Start(ctx context.Context) error {
	if c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is not available")
	}

	channel, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel failed: %w", err)
	}

	if err := channel.ExchangeDeclare(
		c.cfg.RequestedExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		channel.Close()
		return fmt.Errorf("declare requested exchange failed: %w", err)
	}

	queue, err := channel.QueueDeclare(
		c.cfg.RequestedQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return fmt.Errorf("declare requested queue failed: %w", err)
	}

	if err := channel.QueueBind(
		queue.Name,
		c.cfg.RequestedRoutingKey,
		c.cfg.RequestedExchange,
		false,
		nil,
	); err != nil {
		channel.Close()
		return fmt.Errorf("bind requested queue failed: %w", err)
	}

	if err := channel.Qos(1, 0, false); err != nil {
		channel.Close()
		return fmt.Errorf("set qos failed: %w", err)
	}

	deliveries, err := channel.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		return fmt.Errorf("consume from queue failed: %w", err)
	}

	go func() {
		defer channel.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}

				log.Printf("[RequestedConsumer] Mensaje recibido (RoutingKey: %s)", delivery.RoutingKey)

				var event application.SolutionExecutionRequestedEvent
				if err := json.Unmarshal(delivery.Body, &event); err != nil {
					log.Printf("[RequestedConsumer] requested event invalid payload: %v", err)
					_ = delivery.Nack(false, false)
					continue
				}

				log.Printf("[RequestedConsumer] Evento decodificado - EventID: %s, SolutionID: %s", event.EventID, event.Data.SolutionID)

				if err := c.executeUseCase.Execute(ctx, event); err != nil {
					log.Printf("[RequestedConsumer] execute solution use case failed (EventID: %s): %v", event.EventID, err)
					if errors.Is(err, application.ErrInvalidInput) {
						_ = delivery.Nack(false, false)
						continue
					}
					_ = delivery.Nack(false, true)
					continue
				}

				log.Printf("[RequestedConsumer] Evento procesado exitosamente (EventID: %s). Enviando ACK.", event.EventID)
				_ = delivery.Ack(false)
			}
		}
	}()

	return nil
}

var _ application.EventConsumer = (*RequestedConsumer)(nil)
