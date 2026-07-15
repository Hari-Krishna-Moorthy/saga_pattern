package domain

import (
	"context"
	"time"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
	"github.com/google/uuid"
)

// Repository is the persistence port for bookings.
type Repository interface {
	Save(ctx context.Context, b Booking) error
	FindByID(ctx context.Context, id string) (Booking, error)
	Update(ctx context.Context, b Booking) error
}

// Service is the booking-service's saga participant: it owns booking state
// and reacts to events published by the other participants (choreography).
type Service struct {
	repo      Repository
	publisher kafka.Publisher
}

func NewService(repo Repository, publisher kafka.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

type BookingRequestedPayload struct {
	RiderID string `json:"rider_id"`
	Pickup  string `json:"pickup"`
	Dropoff string `json:"dropoff"`
}

// RequestBooking starts the saga: a booking is stored as REQUESTED and a
// booking.requested event is published for the driver-matching service.
func (s *Service) RequestBooking(ctx context.Context, riderID, pickup, dropoff string) (Booking, error) {
	now := time.Now().UTC()
	b := Booking{
		ID:        uuid.NewString(),
		RiderID:   riderID,
		Pickup:    pickup,
		Dropoff:   dropoff,
		Status:    StatusRequested,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Save(ctx, b); err != nil {
		return Booking{}, err
	}

	evt, err := events.NewEnvelope(events.TopicBookingRequested, b.ID, BookingRequestedPayload{
		RiderID: b.RiderID,
		Pickup:  b.Pickup,
		Dropoff: b.Dropoff,
	})
	if err != nil {
		return Booking{}, err
	}

	if err := s.publisher.Publish(ctx, events.TopicBookingRequested, evt); err != nil {
		return Booking{}, err
	}

	return b, nil
}

// HandlePaymentCompleted reacts to payment.completed: the saga has now
// succeeded end-to-end, so the booking is confirmed and a ride.confirmed
// event is published for the notification service.
func (s *Service) HandlePaymentCompleted(ctx context.Context, evt events.Envelope) error {
	b, err := s.repo.FindByID(ctx, evt.BookingID)
	if err != nil {
		return err
	}

	b.Status = StatusConfirmed
	b.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, b); err != nil {
		return err
	}

	confirmedEvt, err := events.NewEnvelope(events.TopicRideConfirmed, b.ID, struct{}{})
	if err != nil {
		return err
	}

	return s.publisher.Publish(ctx, events.TopicRideConfirmed, confirmedEvt)
}
