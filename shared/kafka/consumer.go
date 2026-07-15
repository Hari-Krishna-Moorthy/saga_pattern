package kafka

import (
	"context"
	"errors"
	"log"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	kafkago "github.com/segmentio/kafka-go"
)

// Handler reacts to a single event. Returning an error leaves the message
// unacknowledged so it will be redelivered.
type Handler func(ctx context.Context, evt events.Envelope) error

type Reader struct {
	reader *kafkago.Reader
}

func NewReader(brokers []string, topic, groupID string) *Reader {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
	})
	return &Reader{reader: r}
}

// Run blocks, consuming messages until ctx is cancelled.
func (r *Reader) Run(ctx context.Context, handle Handler) error {
	for {
		msg, err := r.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		evt, err := events.Unmarshal(msg.Value)
		if err != nil {
			log.Printf("skipping malformed message on %s: %v", r.reader.Config().Topic, err)
			_ = r.reader.CommitMessages(ctx, msg)
			continue
		}

		if err := handle(ctx, evt); err != nil {
			log.Printf("handler error on %s for event %s: %v", r.reader.Config().Topic, evt.EventID, err)
			continue
		}

		if err := r.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (r *Reader) Close() error {
	return r.reader.Close()
}
