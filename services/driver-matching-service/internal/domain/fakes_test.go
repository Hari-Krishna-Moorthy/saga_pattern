package domain_test

import (
	"context"
	"sync"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/events"
)

// fakeRepository is an in-memory Repository used by BDD scenarios so they
// exercise real saga-reaction logic without needing Postgres.
type fakeRepository struct {
	mu      sync.Mutex
	drivers map[string]domain.Driver
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{drivers: make(map[string]domain.Driver)}
}

func (r *fakeRepository) addDriver(d domain.Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[d.ID] = d
}

func (r *fakeRepository) FindAvailableDriver(_ context.Context) (domain.Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.drivers {
		if d.Status == domain.DriverAvailable {
			return d, nil
		}
	}
	return domain.Driver{}, domain.ErrNoDriverAvailable
}

func (r *fakeRepository) AssignDriver(_ context.Context, driverID, bookingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.drivers[driverID]
	d.Status = domain.DriverMatched
	d.AssignedBookingID = bookingID
	r.drivers[driverID] = d
	return nil
}

func (r *fakeRepository) driverByID(id string) domain.Driver {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drivers[id]
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
