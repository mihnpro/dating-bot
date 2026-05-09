package rabbitmq

import (
	"context"
	"fmt"

	"github.com/dating-bot/chat-service/internal/domain/event"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Publisher publishes domain events to RabbitMQ topic exchange.
type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	return &Publisher{conn: conn, channel: ch, exchange: exchange}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, evt *event.DomainEvent) error {
	body, err := evt.Marshal()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false,
		amqp.Publishing{ContentType: "application/json", Body: body},
	)
}

func (p *Publisher) Close() {
	_ = p.channel.Close()
	_ = p.conn.Close()
}

// Subscriber receives events from a named queue bound to the topic exchange.
type Subscriber struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewSubscriber(url string) (*Subscriber, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}
	return &Subscriber{conn: conn, channel: ch}, nil
}

func (s *Subscriber) Subscribe(exchange, queue string, handler func([]byte) error) error {
	if err := s.channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	q, err := s.channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := s.channel.QueueBind(q.Name, "#", exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}
	msgs, err := s.channel.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}
	go func() {
		for d := range msgs {
			if err := handler(d.Body); err != nil {
				logrus.WithError(err).Warn("event handler error")
				_ = d.Nack(false, false)
			} else {
				_ = d.Ack(false)
			}
		}
	}()
	return nil
}

func (s *Subscriber) Close() {
	_ = s.channel.Close()
	_ = s.conn.Close()
}
