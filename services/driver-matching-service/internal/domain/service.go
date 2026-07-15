package domain

import (
	"context"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
)

// ErrNoDriverAvailable is returned by Repository.FindAvailableDriver when
// there is no driver currently free to match.
var ErrNoDriverAvailable = errNoDriverAvailable{}

type errNoDriverAvailable struct{}

func (errNoDriverAvailable) Error() string { return "no driver available" }

// Repository is the persistence port for drivers.
type Repository interface {
	// FindAvailableDriver returns an AVAILABLE driver, or ErrNoDriverAvailable.
	FindAvailableDriver(ctx context.Context) (Driver, error)
	AssignDriver(ctx context.Context, driverID, bookingID string) error
	// FindByBookingID returns the driver currently assigned to a booking.
	FindByBookingID(ctx context.Context, bookingID string) (Driver, error)
	ReleaseDriver(ctx context.Context, driverID string) error
}

// Service is the driver-matching-service's saga participant.
type Service struct {
	repo      Repository
	publisher kafka.Publisher
}

func NewService(repo Repository, publisher kafka.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

type DriverMatchedPayload struct {
	DriverID string `json:"driver_id"`
}

// HandleBookingRequested reacts to booking.requested: it tries to assign an
// available driver, publishing driver.matched on success or
// driver.match_failed if none is available.
func (s *Service) HandleBookingRequested(ctx context.Context, evt events.Envelope) error {
	driver, err := s.repo.FindAvailableDriver(ctx)
	if err == ErrNoDriverAvailable {
		failedEvt, evtErr := events.NewEnvelope(events.TopicDriverMatchFailed, evt.BookingID, struct{}{})
		if evtErr != nil {
			return evtErr
		}
		return s.publisher.Publish(ctx, events.TopicDriverMatchFailed, failedEvt)
	}
	if err != nil {
		return err
	}

	if err := s.repo.AssignDriver(ctx, driver.ID, evt.BookingID); err != nil {
		return err
	}

	matchedEvt, err := events.NewEnvelope(events.TopicDriverMatched, evt.BookingID, DriverMatchedPayload{DriverID: driver.ID})
	if err != nil {
		return err
	}
	return s.publisher.Publish(ctx, events.TopicDriverMatched, matchedEvt)
}

// HandlePaymentFailed is the compensation for a matched driver: payment
// could not be taken, so the driver reserved for this booking is released
// back to the pool.
func (s *Service) HandlePaymentFailed(ctx context.Context, evt events.Envelope) error {
	driver, err := s.repo.FindByBookingID(ctx, evt.BookingID)
	if err != nil {
		return err
	}

	if err := s.repo.ReleaseDriver(ctx, driver.ID); err != nil {
		return err
	}

	releasedEvt, err := events.NewEnvelope(events.TopicDriverReleased, evt.BookingID, struct{}{})
	if err != nil {
		return err
	}
	return s.publisher.Publish(ctx, events.TopicDriverReleased, releasedEvt)
}
