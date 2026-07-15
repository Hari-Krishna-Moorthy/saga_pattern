package domain

import (
	"context"

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
