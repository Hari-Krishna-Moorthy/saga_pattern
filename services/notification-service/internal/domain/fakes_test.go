package domain_test

import (
	"context"
	"sync"
)

// fakeNotifier is an in-memory Notifier used by BDD scenarios so they
// exercise real saga-reaction logic without needing a real notification
// channel.
type fakeNotifier struct {
	mu   sync.Mutex
	sent []sentNotification
}

type sentNotification struct {
	BookingID string
	Message   string
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{}
}

func (n *fakeNotifier) Send(_ context.Context, bookingID, message string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, sentNotification{BookingID: bookingID, Message: message})
	return nil
}

func (n *fakeNotifier) notificationsForBooking(bookingID string) []sentNotification {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []sentNotification
	for _, s := range n.sent {
		if s.BookingID == bookingID {
			out = append(out, s)
		}
	}
	return out
}
