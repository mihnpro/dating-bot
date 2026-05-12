package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// Subscriber receives raw event bytes from a named queue bound to a topic exchange.
// It reconnects automatically when the AMQP connection is dropped.
type Subscriber struct {
	url  string
	done chan struct{}
}

func NewSubscriber(url string) (*Subscriber, error) {
	// Validate connectivity at startup so the container restarts if RabbitMQ is absent.
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}
	conn.Close()
	return &Subscriber{url: url, done: make(chan struct{})}, nil
}

// Subscribe binds the queue to the exchange and starts a goroutine that consumes
// messages and reconnects automatically when the connection is lost.
func (s *Subscriber) Subscribe(queue, exchange string, handler func(body []byte) error) error {
	msgs, conn, ch, err := s.openConsumer(queue, exchange)
	if err != nil {
		return err
	}
	go s.loop(queue, exchange, handler, msgs, conn, ch)
	return nil
}

func (s *Subscriber) openConsumer(queue, exchange string) (<-chan amqp.Delivery, *amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(s.url)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("set qos: %w", err)
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("declare exchange %q: %w", exchange, err)
	}
	q, err := ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("declare queue %q: %w", queue, err)
	}
	if err := ch.QueueBind(q.Name, "#", exchange, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("bind queue %q: %w", queue, err)
	}
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("start consuming %q: %w", queue, err)
	}
	return msgs, conn, ch, nil
}

func (s *Subscriber) loop(
	queue, exchange string,
	handler func(body []byte) error,
	msgs <-chan amqp.Delivery,
	conn *amqp.Connection,
	ch *amqp.Channel,
) {
	backoff := initialBackoff
	for {
		// Drain the channel until it closes (connection dropped or server-side cancel).
		for msg := range msgs {
			if err := handler(msg.Body); err != nil {
				logrus.WithError(err).Warn("notification event handler error — nack")
				_ = msg.Nack(false, true)
			} else {
				_ = msg.Ack(false)
			}
		}
		ch.Close()
		conn.Close()

		select {
		case <-s.done:
			return
		default:
			logrus.Warn("RabbitMQ connection lost — reconnecting")
		}

		// Reconnect loop with exponential backoff.
		for {
			select {
			case <-s.done:
				return
			case <-time.After(backoff):
			}

			var err error
			msgs, conn, ch, err = s.openConsumer(queue, exchange)
			if err != nil {
				logrus.WithError(err).Warnf("RabbitMQ reconnect failed — next retry in %s", backoff)
				if backoff < maxBackoff {
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
				continue
			}

			backoff = initialBackoff
			logrus.Info("RabbitMQ reconnected")
			break
		}
	}
}

func (s *Subscriber) Close() error {
	close(s.done)
	return nil
}
