package domain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/cucumber/godog"
)

// matchingTestCtx carries state across the steps of a single scenario.
type matchingTestCtx struct {
	repo      *fakeRepository
	publisher *fakePublisher
	service   *domain.Service

	bookingID string
	lastErr   error
}

func (t *matchingTestCtx) reset() {
	t.repo = newFakeRepository()
	t.publisher = newFakePublisher()
	t.service = domain.NewService(t.repo, t.publisher)
}

func (t *matchingTestCtx) aDriverIsAvailable(driverID string) error {
	t.repo.addDriver(domain.Driver{ID: driverID, Status: domain.DriverAvailable})
	return nil
}

func (t *matchingTestCtx) noDriversAreAvailable() error {
	return nil // fake repository starts empty; nothing to do
}

func (t *matchingTestCtx) bookingIsRequested(bookingID string) error {
	t.bookingID = bookingID
	evt, err := events.NewEnvelope(events.TopicBookingRequested, bookingID, struct{}{})
	if err != nil {
		return err
	}
	t.lastErr = t.service.HandleBookingRequested(context.Background(), evt)
	return t.lastErr
}

func (t *matchingTestCtx) driverShouldBeAssignedToBooking(driverID, bookingID string) error {
	d := t.repo.driverByID(driverID)
	if d.Status != domain.DriverMatched {
		return fmt.Errorf("expected driver %s to be MATCHED, got %s", driverID, d.Status)
	}
	if d.AssignedBookingID != bookingID {
		return fmt.Errorf("expected driver %s assigned to booking %s, got %s", driverID, bookingID, d.AssignedBookingID)
	}
	return nil
}

func (t *matchingTestCtx) anEventShouldBePublishedForBooking(topic, bookingID string) error {
	published := t.publisher.eventsOnTopic(topic)
	for _, evt := range published {
		if evt.BookingID == bookingID {
			return nil
		}
	}
	return fmt.Errorf("expected a %q event for booking %s, got none (topic events: %d)", topic, bookingID, len(published))
}

func InitializeScenario(sc *godog.ScenarioContext) {
	t := &matchingTestCtx{}

	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		t.reset()
		return ctx, nil
	})

	sc.Step(`^a driver "([^"]*)" is available$`, t.aDriverIsAvailable)
	sc.Step(`^no drivers are available$`, t.noDriversAreAvailable)
	sc.Step(`^booking "([^"]*)" is requested$`, t.bookingIsRequested)
	sc.Step(`^driver "([^"]*)" should be assigned to booking "([^"]*)"$`, t.driverShouldBeAssignedToBooking)
	sc.Step(`^a "([^"]*)" event should be published for booking "([^"]*)"$`, t.anEventShouldBePublishedForBooking)
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
