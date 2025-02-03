package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client represents a RabbitMQ client
type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  Config
}

// Config holds RabbitMQ configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
}

// New creates a new RabbitMQ client
func New(cfg Config) (*Client, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.VHost)

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %v", err)
	}

	return &Client{
		conn:    conn,
		channel: ch,
		config:  cfg,
	}, nil
}

// DeclareQueue declares a queue with the given name and options
func (c *Client) DeclareQueue(name string, durable, autoDelete bool) (amqp.Queue, error) {
	return c.channel.QueueDeclare(
		name,       // name
		durable,    // durable
		autoDelete, // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
}

// Publish publishes a message to a queue
func (c *Client) Publish(ctx context.Context, exchange, routingKey string, msg []byte) error {
	return c.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        msg,
			Timestamp:   time.Now(),
		},
	)
}

// Consume starts consuming messages from a queue
func (c *Client) Consume(queueName string, handler func([]byte) error) error {
	msgs, err := c.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	go func() {
		for msg := range msgs {
			if err := handler(msg.Body); err != nil {
				log.Printf("Error processing message: %v", err)
				msg.Nack(false, true) // negative acknowledgement, requeue
				continue
			}
			msg.Ack(false) // acknowledge message
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection and channel
func (c *Client) Close() error {
	if err := c.channel.Close(); err != nil {
		return fmt.Errorf("failed to close channel: %v", err)
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %v", err)
	}
	return nil
}

// HealthCheck performs a health check on the RabbitMQ connection
func (c *Client) HealthCheck() error {
	if c.conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}
	return nil
}

// Reconnect attempts to reconnect to RabbitMQ
func (c *Client) Reconnect() error {
	if err := c.Close(); err != nil {
		log.Printf("Error closing existing connection: %v", err)
	}

	client, err := New(c.config)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %v", err)
	}

	c.conn = client.conn
	c.channel = client.channel
	return nil
}
