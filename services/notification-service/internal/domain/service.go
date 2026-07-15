package domain

import (
	"context"
	"encoding/json"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
)

// Notifier is the port to wherever notifications actually go (push, SMS,
// email...). This demo only needs to know that a message was sent.
type Notifier interface {
	Send(ctx context.Context, bookingID, message string) error
}

type Service struct {
	notifier Notifier
}

func NewService(notifier Notifier) *Service {
	return &Service{notifier: notifier}
}

// HandleRideConfirmed reacts to ride.confirmed: the saga completed
// successfully, so the rider is told their ride is on.
func (s *Service) HandleRideConfirmed(ctx context.Context, evt events.Envelope) error {
	return s.notifier.Send(ctx, evt.BookingID, "Your ride is confirmed!")
}

type bookingCancelledPayload struct {
	Reason string `json:"reason"`
}

// HandleBookingCancelled reacts to booking.cancelled: the saga could not
// complete, so the rider is told their ride was cancelled and why.
func (s *Service) HandleBookingCancelled(ctx context.Context, evt events.Envelope) error {
	var payload bookingCancelledPayload
	_ = json.Unmarshal(evt.Payload, &payload) // reason is best-effort context

	message := "Your ride was cancelled."
	if payload.Reason != "" {
		message = "Your ride was cancelled: " + payload.Reason
	}
	return s.notifier.Send(ctx, evt.BookingID, message)
}
