package messaging

import (
	"context"
	"log"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/idempotency"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
)

const groupID = "booking-service"

// Run starts one Kafka consumer goroutine per subscribed topic: payment.completed
// (happy path), driver.match_failed and payment.failed (compensations). It blocks
// until ctx is cancelled.
func Run(ctx context.Context, brokers []string, svc *domain.Service, idem idempotency.Checker) {
	handlers := map[string]func(context.Context, events.Envelope) error{
		events.TopicPaymentCompleted:  svc.HandlePaymentCompleted,
		events.TopicDriverMatchFailed: svc.HandleDriverMatchFailed,
		events.TopicPaymentFailed:     svc.HandlePaymentFailed,
	}

	for topic, handle := range handlers {
		go func(topic string, handle kafka.Handler) {
			reader := kafka.NewReader(brokers, topic, groupID)
			defer reader.Close()

			err := reader.Run(ctx, func(ctx context.Context, evt events.Envelope) error {
				seen, err := idem.AlreadyProcessed(ctx, evt.EventID)
				if err != nil {
					return err
				}
				if seen {
					log.Printf("skipping already-processed event %s on %s", evt.EventID, topic)
					return nil
				}
				return handle(ctx, evt)
			})
			if err != nil {
				log.Printf("consumer for %s stopped: %v", topic, err)
			}
		}(topic, handle)
	}
}
