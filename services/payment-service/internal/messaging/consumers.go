package messaging

import (
	"context"
	"log"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/idempotency"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
)

const groupID = "payment-service"

// Run consumes driver.matched and reacts by charging the rider. It blocks
// until ctx is cancelled.
func Run(ctx context.Context, brokers []string, svc *domain.Service, idem idempotency.Checker) {
	reader := kafka.NewReader(brokers, events.TopicDriverMatched, groupID)
	defer reader.Close()

	err := reader.Run(ctx, func(ctx context.Context, evt events.Envelope) error {
		seen, err := idem.AlreadyProcessed(ctx, evt.EventID)
		if err != nil {
			return err
		}
		if seen {
			log.Printf("skipping already-processed event %s on %s", evt.EventID, events.TopicDriverMatched)
			return nil
		}
		return svc.HandleDriverMatched(ctx, evt)
	})
	if err != nil {
		log.Printf("consumer for %s stopped: %v", events.TopicDriverMatched, err)
	}
}
