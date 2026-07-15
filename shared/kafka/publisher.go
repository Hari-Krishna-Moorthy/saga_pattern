package kafka

import (
	"context"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	kafkago "github.com/segmentio/kafka-go"
)

// Publisher is the port every service's domain/application layer depends on.
// The real implementation talks to Kafka; BDD tests use an in-memory fake.
type Publisher interface {
	Publish(ctx context.Context, topic string, evt events.Envelope) error
}

type Writer struct {
	brokers []string
	writers map[string]*kafkago.Writer
}

func NewWriter(brokers []string) *Writer {
	return &Writer{brokers: brokers, writers: make(map[string]*kafkago.Writer)}
}

func (w *Writer) Publish(ctx context.Context, topic string, evt events.Envelope) error {
	writer, ok := w.writers[topic]
	if !ok {
		writer = &kafkago.Writer{
			Addr:                   kafkago.TCP(w.brokers...),
			Topic:                  topic,
			Balancer:               &kafkago.LeastBytes{},
			AllowAutoTopicCreation: true,
		}
		w.writers[topic] = writer
	}

	data, err := evt.Marshal()
	if err != nil {
		return err
	}

	return writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(evt.BookingID),
		Value: data,
	})
}

func (w *Writer) Close() error {
	for _, writer := range w.writers {
		if err := writer.Close(); err != nil {
			return err
		}
	}
	return nil
}
