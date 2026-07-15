package domain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service/internal/domain"
	"github.com/cucumber/godog"
)

// bookingTestCtx carries state across the steps of a single scenario.
type bookingTestCtx struct {
	repo      *fakeRepository
	publisher *fakePublisher
	service   *domain.Service

	riderID string
	booking domain.Booking
	lastErr error
}

func (t *bookingTestCtx) reset() {
	t.repo = newFakeRepository()
	t.publisher = newFakePublisher()
	t.service = domain.NewService(t.repo, t.publisher)
}

func (t *bookingTestCtx) iAmARiderWithID(riderID string) error {
	t.riderID = riderID
	return nil
}

func (t *bookingTestCtx) iRequestARideFromTo(pickup, dropoff string) error {
	b, err := t.service.RequestBooking(context.Background(), t.riderID, pickup, dropoff)
	t.booking = b
	t.lastErr = err
	return err
}

func (t *bookingTestCtx) theBookingShouldBeStoredWithStatus(status string) error {
	stored, err := t.repo.FindByID(context.Background(), t.booking.ID)
	if err != nil {
		return err
	}
	if string(stored.Status) != status {
		return fmt.Errorf("expected booking status %q, got %q", status, stored.Status)
	}
	return nil
}

func (t *bookingTestCtx) anEventShouldBePublishedForTheBooking(topic string) error {
	published := t.publisher.eventsOnTopic(topic)
	for _, evt := range published {
		if evt.BookingID == t.booking.ID {
			return nil
		}
	}
	return fmt.Errorf("expected a %q event for booking %s, got none (topic events: %d)", topic, t.booking.ID, len(published))
}

func InitializeScenario(sc *godog.ScenarioContext) {
	t := &bookingTestCtx{}

	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		t.reset()
		return ctx, nil
	})

	sc.Step(`^I am a rider with id "([^"]*)"$`, t.iAmARiderWithID)
	sc.Step(`^I request a ride from "([^"]*)" to "([^"]*)"$`, t.iRequestARideFromTo)
	sc.Step(`^the booking should be stored with status "([^"]*)"$`, t.theBookingShouldBeStoredWithStatus)
	sc.Step(`^a "([^"]*)" event should be published for the booking$`, t.anEventShouldBePublishedForTheBooking)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../../features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
