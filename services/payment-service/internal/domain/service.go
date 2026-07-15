package domain

import (
	"context"
	"encoding/json"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
	"github.com/google/uuid"
)

// ErrInsufficientFunds is returned by Gateway.Charge when the rider's
// payment method can't cover the fare.
var ErrInsufficientFunds = errInsufficientFunds{}

type errInsufficientFunds struct{}

func (errInsufficientFunds) Error() string { return "insufficient funds" }

// Gateway is the port to the payment processor.
type Gateway interface {
	// Charge attempts to take payment for a booking. It returns
	// ErrInsufficientFunds if the charge is declined for that reason.
	Charge(ctx context.Context, bookingID string) error
}

// Repository is the persistence port for payment records.
type Repository interface {
	Save(ctx context.Context, p Payment) error
}

type DriverMatchedPayload struct {
	DriverID string `json:"driver_id"`
}

// Service is the payment-service's saga participant.
type Service struct {
	repo      Repository
	gateway   Gateway
	publisher kafka.Publisher
}

func NewService(repo Repository, gateway Gateway, publisher kafka.Publisher) *Service {
	return &Service{repo: repo, gateway: gateway, publisher: publisher}
}

// HandleDriverMatched reacts to driver.matched: a driver has been reserved,
// so this is the saga's cue to take payment. It publishes payment.completed
// on success or payment.failed (triggering compensation) if the charge is
// declined.
func (s *Service) HandleDriverMatched(ctx context.Context, evt events.Envelope) error {
	var payload DriverMatchedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return err
	}

	chargeErr := s.gateway.Charge(ctx, evt.BookingID)
	if chargeErr == ErrInsufficientFunds {
		p := Payment{ID: uuid.NewString(), BookingID: evt.BookingID, DriverID: payload.DriverID, Status: StatusFailed}
		if err := s.repo.Save(ctx, p); err != nil {
			return err
		}
		failedEvt, err := events.NewEnvelope(events.TopicPaymentFailed, evt.BookingID, struct{}{})
		if err != nil {
			return err
		}
		return s.publisher.Publish(ctx, events.TopicPaymentFailed, failedEvt)
	}
	if chargeErr != nil {
		return chargeErr
	}

	p := Payment{ID: uuid.NewString(), BookingID: evt.BookingID, DriverID: payload.DriverID, Status: StatusCompleted}
	if err := s.repo.Save(ctx, p); err != nil {
		return err
	}

	completedEvt, err := events.NewEnvelope(events.TopicPaymentCompleted, evt.BookingID, struct{}{})
	if err != nil {
		return err
	}
	return s.publisher.Publish(ctx, events.TopicPaymentCompleted, completedEvt)
}
