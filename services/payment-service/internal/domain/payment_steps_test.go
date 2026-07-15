package domain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/cucumber/godog"
)

// paymentTestCtx carries state across the steps of a single scenario.
type paymentTestCtx struct {
	repo      *fakeRepository
	gateway   *fakeGateway
	publisher *fakePublisher
	service   *domain.Service

	lastErr error
}

func (t *paymentTestCtx) reset() {
	t.repo = newFakeRepository()
	t.gateway = &fakeGateway{}
	t.publisher = newFakePublisher()
	t.service = domain.NewService(t.repo, t.gateway, t.publisher)
}

func (t *paymentTestCtx) theRidersPaymentMethodWillBeAccepted() error {
	t.gateway.declineAll = false
	return nil
}

func (t *paymentTestCtx) theRidersPaymentMethodWillBeDeclinedForInsufficientFunds() error {
	t.gateway.declineAll = true
	return nil
}

func (t *paymentTestCtx) driverIsMatchedToBooking(driverID, bookingID string) error {
	evt, err := events.NewEnvelope(events.TopicDriverMatched, bookingID, domain.DriverMatchedPayload{DriverID: driverID})
	if err != nil {
		return err
	}
	t.lastErr = t.service.HandleDriverMatched(context.Background(), evt)
	return t.lastErr
}

func (t *paymentTestCtx) paymentForBookingShouldBeRecordedAs(bookingID, status string) error {
	p, ok := t.repo.paymentForBooking(bookingID)
	if !ok {
		return fmt.Errorf("expected a payment recorded for booking %s, found none", bookingID)
	}
	if string(p.Status) != status {
		return fmt.Errorf("expected payment status %q, got %q", status, p.Status)
	}
	return nil
}

func (t *paymentTestCtx) anEventShouldBePublishedForBooking(topic, bookingID string) error {
	published := t.publisher.eventsOnTopic(topic)
	for _, evt := range published {
		if evt.BookingID == bookingID {
			return nil
		}
	}
	return fmt.Errorf("expected a %q event for booking %s, got none (topic events: %d)", topic, bookingID, len(published))
}

func InitializeScenario(sc *godog.ScenarioContext) {
	t := &paymentTestCtx{}

	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		t.reset()
		return ctx, nil
	})

	sc.Step(`^the rider's payment method will be accepted$`, t.theRidersPaymentMethodWillBeAccepted)
	sc.Step(`^the rider's payment method will be declined for insufficient funds$`, t.theRidersPaymentMethodWillBeDeclinedForInsufficientFunds)
	sc.Step(`^driver "([^"]*)" is matched to booking "([^"]*)"$`, t.driverIsMatchedToBooking)
	sc.Step(`^a payment for booking "([^"]*)" should be recorded as "([^"]*)"$`, t.paymentForBookingShouldBeRecordedAs)
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
