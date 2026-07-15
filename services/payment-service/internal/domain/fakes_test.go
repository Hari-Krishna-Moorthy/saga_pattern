package domain_test

import (
	"context"
	"sync"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
)

// fakeRepository is an in-memory Repository used by BDD scenarios so they
// exercise real saga-reaction logic without needing Postgres.
type fakeRepository struct {
	mu       sync.Mutex
	payments []domain.Payment
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{}
}

func (r *fakeRepository) Save(_ context.Context, p domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payments = append(r.payments, p)
	return nil
}

func (r *fakeRepository) paymentForBooking(bookingID string) (domain.Payment, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		if p.BookingID == bookingID {
			return p, true
		}
	}
	return domain.Payment{}, false
}

// fakeGateway is a configurable payment gateway for scenarios to drive
// either the success or the insufficient-funds path.
type fakeGateway struct {
	declineAll bool
}

func (g *fakeGateway) Charge(_ context.Context, _ string) error {
	if g.declineAll {
		return domain.ErrInsufficientFunds
	}
	return nil
}

// fakePublisher is an in-memory kafka.Publisher that records every event
// published, so scenarios can assert on saga output events.
type fakePublisher struct {
	mu        sync.Mutex
	published []publishedEvent
}

type publishedEvent struct {
	Topic string
	Event events.Envelope
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{}
}

func (p *fakePublisher) Publish(_ context.Context, topic string, evt events.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, publishedEvent{Topic: topic, Event: evt})
	return nil
}

func (p *fakePublisher) eventsOnTopic(topic string) []events.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []events.Envelope
	for _, pe := range p.published {
		if pe.Topic == topic {
			out = append(out, pe.Event)
		}
	}
	return out
}
