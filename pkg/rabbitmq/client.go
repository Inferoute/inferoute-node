package rabbitmq

import (
	"fmt"

	"github.com/streadway/amqp"
)

// Client represents a RabbitMQ client
type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// DeclareQueue declares a queue and binds it to an exchange
func (c *Client) DeclareQueue(queueName, exchangeName, routingKey string) error {
	// Declare the queue
	queue, err := c.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	// Bind the queue to the exchange
	err = c.channel.QueueBind(
		queue.Name,   // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, exchangeName, err)
	}

	return nil
}

// DeclareExchange declares a new exchange
func (c *Client) DeclareExchange(name, kind string) error {
	return c.channel.ExchangeDeclare(
		name,  // name
		kind,  // type
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
}

// NewClient creates a new RabbitMQ client
func NewClient(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	client := &Client{
		conn:    conn,
		channel: ch,
	}

	// Declare the provider health exchange
	if err := client.DeclareExchange("provider_health", "topic"); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare and bind the health updates queue
	if err := client.DeclareQueue(
		"provider_health_updates",
		"provider_health",
		"health_updates",
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to setup queue: %w", err)
	}

	return client, nil
}

// Close closes the RabbitMQ connection and channel
func (c *Client) Close() error {
	if err := c.channel.Close(); err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}

// Publish publishes a message to RabbitMQ
func (c *Client) Publish(exchange, routingKey string, body []byte) error {
	return c.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// MessageHandler is a function that processes a message
type MessageHandler func([]byte) error

// Consume starts consuming messages from a queue
func (c *Client) Consume(exchange, routingKey, queueName string, handler MessageHandler) error {
	// Declare the exchange
	err := c.channel.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the queue
	q, err := c.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind the queue to the exchange
	err = c.channel.QueueBind(
		q.Name,     // queue name
		routingKey, // routing key
		exchange,   // exchange
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Start consuming messages
	msgs, err := c.channel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	fmt.Printf("Started consuming messages from queue: %s\n", queueName)

	go func() {
		defer fmt.Printf("Consumer stopped for queue: %s\n", queueName)

		for d := range msgs {
			fmt.Printf("Received message from queue: %s\n", queueName)

			select {
			case <-c.channel.NotifyClose(make(chan *amqp.Error)):
				fmt.Printf("Channel closed, stopping consumer for queue: %s\n", queueName)
				return
			default:
				if err := handler(d.Body); err != nil {
					fmt.Printf("Error processing message: %v\n", err)
					// Nack the message to requeue it
					if nackErr := d.Nack(false, true); nackErr != nil {
						fmt.Printf("Error nacking message: %v\n", nackErr)
					} else {
						fmt.Printf("Message nacked and requeued\n")
					}
					continue
				}

				// Acknowledge the message
				if ackErr := d.Ack(false); ackErr != nil {
					fmt.Printf("Error acking message: %v\n", ackErr)
				} else {
					fmt.Printf("Message successfully acked\n")
				}
			}
		}
	}()

	return nil
}
