package messaging

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func NewConnection(rabbitURL string) (*amqp.Connection, error) {
	log.Printf("Iniciando conexión a RabbitMQ...")
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial failed: %w", err)
	}

	log.Printf("Verificando canal de RabbitMQ...")
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel test failed: %w", err)
	}
	_ = ch.Close()

	log.Printf("Conexión a RabbitMQ establecida con éxito")
	return conn, nil
}

func ConsumeEvents(conn *amqp.Connection, queueName string, handler func([]byte) error) error {
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare a queue: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	log.Printf("Esperando eventos en la cola: %s", queueName)

	go func() {
		for d := range msgs {
			log.Printf("Evento recibido de RabbitMQ [Cola: %s]: %s", queueName, string(d.Body))

			log.Printf("Iniciando fase de ejecución para el evento...")
			if err := handler(d.Body); err != nil {
				log.Printf("Error durante la fase de ejecución: %v", err)
			} else {
				log.Printf("Fase de ejecución completada con éxito")
			}
		}
	}()

	return nil
}
