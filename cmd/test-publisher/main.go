package main

import (
	"encoding/json"
	"log"
	"time"

	
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	rabbitURL := "amqp://rabbitmq:rabbitmq@localhost:5672/"
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("rabbitmq dial failed: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("rabbitmq channel failed: %v", err)
	}
	defer ch.Close()

	exchange := "challenges.solutions.exchange"
	routingKey := "challenges.solution.execution.requested"

	// Create exchange if not exists
	err = ch.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("exchange declare failed: %v", err)
	}

	publishPythonEvent(ch, exchange, routingKey)
	publishJSEvent(ch, exchange, routingKey)
}

func publishPythonEvent(ch *amqp.Channel, exchange, routingKey string) {
	eventId := "uuid-test-" + time.Now().Format("150405.000")
	solutionId := "uuid-test-" + time.Now().Format("150405.000")
	testId1 := "uuid-test-" + time.Now().Format("150405.000")
	testId2 := "uuid-test-" + time.Now().Format("150405.000")

	event := map[string]interface{}{
		"eventId":   eventId,
		"eventType": "SolutionExecutionRequestedEvent",
		"timestamp": time.Now().Format(time.RFC3339),
		"data": map[string]interface{}{
			"solutionId":        solutionId,
			"language":          "python",
			"entryFunctionName": "fibonacci",
			"code": `def fibonacci(n):
    if n == 0:
        return 0
    elif n == 1:
        return 1
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b`,
			"testCases": []map[string]interface{}{
				{
					"testId":         testId1,
					"input":          "0",
					"expectedOutput": "0",
					"isHidden":       false,
				},
				{
					"testId":         testId2,
					"input":          "10",
					"expectedOutput": "55",
					"isHidden":       false,
				},
			},
		},
	}

	body, _ := json.Marshal(event)
	err := ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Printf("Failed to publish Python event: %v", err)
	} else {
		log.Printf("Successfully published Python event: EventID=%s", eventId)
	}
}

func publishJSEvent(ch *amqp.Channel, exchange, routingKey string) {
	eventId := "uuid-test-" + time.Now().Format("150405.000")
	solutionId := "uuid-test-" + time.Now().Format("150405.000")
	testId1 := "uuid-test-" + time.Now().Format("150405.000")
	testId2 := "uuid-test-" + time.Now().Format("150405.000")

	event := map[string]interface{}{
		"eventId":   eventId,
		"eventType": "SolutionExecutionRequestedEvent",
		"timestamp": time.Now().Format(time.RFC3339),
		"data": map[string]interface{}{
			"solutionId":        solutionId,
			"language":          "javascript",
			"entryFunctionName": "fibonacci",
			"code": `function fibonacci(n) {
    if (n === 0) return 0;
    if (n === 1) return 1;
    let a = 0, b = 1;
    for (let i = 2; i <= n; i++) {
        let temp = a + b;
        a = b;
        b = temp;
    }
    return b;
}`,
			"testCases": []map[string]interface{}{
				{
					"testId":         testId1,
					"input":          "0",
					"expectedOutput": "0",
					"isHidden":       false,
				},
				{
					"testId":         testId2,
					"input":          "10",
					"expectedOutput": "55",
					"isHidden":       false,
				},
			},
		},
	}

	body, _ := json.Marshal(event)
	err := ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Printf("Failed to publish JS event: %v", err)
	} else {
		log.Printf("Successfully published JS event: EventID=%s", eventId)
	}
}
