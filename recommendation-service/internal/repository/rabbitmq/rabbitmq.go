package rabbitmq

import (
	"context"
	"fmt"

	"github.com/dating-bot/recommendation-service/internal/domain/entity"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ── Publisher ─────────────────────────────────────────────────────────────────

type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(
		exchange,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange %q: %w", exchange, err)
	}

	return &Publisher{conn: conn, channel: ch, exchange: exchange}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, evt *entity.DomainEvent) error {
	body, err := evt.Marshal()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.channel.PublishWithContext(ctx,
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

func (p *Publisher) Close() error {
	if err := p.channel.Close(); err != nil {
		return err
	}
	return p.conn.Close()
}

// ── Subscriber ────────────────────────────────────────────────────────────────

type Subscriber struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewSubscriber(url string) (*Subscriber, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq open channel: %w", err)
	}

	// Prefetch one message at a time so the worker pool controls throughput.
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	return &Subscriber{conn: conn, channel: ch}, nil
}

// Subscribe binds queue to exchange with the given routing key pattern and
// starts consuming in a background goroutine.
// handler is called synchronously inside that goroutine; return a non-nil
// error to nack-and-requeue, return nil to ack.
func (s *Subscriber) Subscribe(
	queue, exchange string,
	handler func(evt *entity.DomainEvent) error,
) error {
	if err := s.channel.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange %q: %w", exchange, err)
	}

	q, err := s.channel.QueueDeclare(
		queue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue %q: %w", queue, err)
	}

	// Bind to all routing keys on this exchange so the service receives every
	// event and can filter by event_name in the handler.
	if err := s.channel.QueueBind(q.Name, "#", exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %q to exchange %q: %w", queue, exchange, err)
	}

	msgs, err := s.channel.Consume(
		q.Name,
		"",    // consumer tag — auto-generated
		false, // auto-ack — we ack/nack manually
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consuming %q: %w", queue, err)
	}

	go func() {
		for msg := range msgs {
			evt, err := entity.UnmarshalDomainEvent(msg.Body)
			if err != nil {
				// Malformed message — dead-letter without requeue.
				_ = msg.Nack(false, false)
				continue
			}

			if err := handler(evt); err != nil {
				// Transient error — requeue for retry.
				_ = msg.Nack(false, true)
				continue
			}

			_ = msg.Ack(false)
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
