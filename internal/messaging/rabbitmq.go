package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func NewConnection(rabbitURL string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel test failed: %w", err)
	}
	_ = ch.Close()

	return conn, nil
}
