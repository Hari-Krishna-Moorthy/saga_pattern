package domain_test

import (
	"context"
	"fmt"
	"sync"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
)

// fakeRepository is an in-memory Repository used by BDD scenarios so they
// exercise real saga-reaction logic without needing Postgres.
type fakeRepository struct {
	mu       sync.Mutex
	bookings map[string]domain.Booking
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{bookings: make(map[string]domain.Booking)}
}

func (r *fakeRepository) Save(_ context.Context, b domain.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bookings[b.ID] = b
	return nil
}

func (r *fakeRepository) FindByID(_ context.Context, id string) (domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bookings[id]
	if !ok {
		return domain.Booking{}, fmt.Errorf("booking %s not found", id)
	}
	return b, nil
}

func (r *fakeRepository) Update(_ context.Context, b domain.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.bookings[b.ID]; !ok {
		return fmt.Errorf("booking %s not found", b.ID)
	}
	r.bookings[b.ID] = b
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
