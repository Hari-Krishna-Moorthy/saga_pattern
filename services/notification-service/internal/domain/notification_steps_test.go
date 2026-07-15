package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/notification-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
	"github.com/cucumber/godog"
)

// notificationTestCtx carries state across the steps of a single scenario.
type notificationTestCtx struct {
	notifier *fakeNotifier
	service  *domain.Service

	lastErr error
}

func (t *notificationTestCtx) reset() {
	t.notifier = newFakeNotifier()
	t.service = domain.NewService(t.notifier)
}

func (t *notificationTestCtx) theRideIsConfirmedForBooking(bookingID string) error {
	evt, err := events.NewEnvelope(events.TopicRideConfirmed, bookingID, struct{}{})
	if err != nil {
		return err
	}
	t.lastErr = t.service.HandleRideConfirmed(context.Background(), evt)
	return t.lastErr
}

func (t *notificationTestCtx) bookingIsCancelledWithReason(bookingID, reason string) error {
	evt, err := events.NewEnvelope(events.TopicBookingCancelled, bookingID, struct {
		Reason string `json:"reason"`
	}{Reason: reason})
	if err != nil {
		return err
	}
	t.lastErr = t.service.HandleBookingCancelled(context.Background(), evt)
	return t.lastErr
}

func (t *notificationTestCtx) theRiderShouldBeNotifiedForBookingWithMessageContaining(bookingID, needle string) error {
	notifications := t.notifier.notificationsForBooking(bookingID)
	for _, n := range notifications {
		if strings.Contains(n.Message, needle) {
			return nil
		}
	}
	return fmt.Errorf("expected a notification for booking %s containing %q, got %v", bookingID, needle, notifications)
}

func InitializeScenario(sc *godog.ScenarioContext) {
	t := &notificationTestCtx{}

	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		t.reset()
		return ctx, nil
	})

	sc.Step(`^the ride is confirmed for booking "([^"]*)"$`, t.theRideIsConfirmedForBooking)
	sc.Step(`^booking "([^"]*)" is cancelled with reason "([^"]*)"$`, t.bookingIsCancelledWithReason)
	sc.Step(`^the rider should be notified for booking "([^"]*)" with a message containing "([^"]*)"$`, t.theRiderShouldBeNotifiedForBookingWithMessageContaining)
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
