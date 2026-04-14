package rabbitmq

import (
	"context"
	"fmt"

	"github.com/dating-bot/user-profile-service/internal/domain/event"
	amqp "github.com/rabbitmq/amqp091-go"
)

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

	if err := ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // args
	); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	return &Publisher{
		conn:     conn,
		channel:  ch,
		exchange: exchange,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, evt *event.DomainEvent) error {
	body, err := evt.Marshal()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.channel.PublishWithContext(ctx,
		p.exchange, // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (p *Publisher) Close() error {
	if err := p.channel.Close(); err != nil {
		return err
	}
	return p.conn.Close()
}

// Subscriber handles consuming events from RabbitMQ
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

	return &Subscriber{
		conn:    conn,
		channel: ch,
	}, nil
}

func (s *Subscriber) Consume(queue, exchange string, handler func(evt *event.DomainEvent) error) error {
	if err := s.channel.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	q, err := s.channel.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
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
		for msg := range msgs {
			evt, err := event.UnmarshalDomainEvent(msg.Body)
			if err != nil {
				msg.Nack(false, false)
				continue
			}
			if err := handler(evt); err != nil {
				msg.Nack(false, true)
				continue
			}
			msg.Ack(false)
		}
	}()

	return nil
}

func (s *Subscriber) Close() error {
	if err := s.channel.Close(); err != nil {
		return err
	}
	return s.conn.Close()
}
