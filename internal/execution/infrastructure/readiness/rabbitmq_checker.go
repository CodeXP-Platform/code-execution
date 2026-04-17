package readiness

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQChecker struct {
	conn *amqp.Connection
}

func NewRabbitMQChecker(conn *amqp.Connection) RabbitMQChecker {
	return RabbitMQChecker{conn: conn}
}

func (c RabbitMQChecker) Name() string {
	return "rabbitmq"
}

func (c RabbitMQChecker) Check(_ context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("rabbitmq connection is nil")
	}
	if c.conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}
	return nil
}
