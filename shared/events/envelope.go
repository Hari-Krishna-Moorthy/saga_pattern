package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope is the wire format for every event published across the saga.
// EventID is used as the idempotency key by consumers.
type Envelope struct {
	EventID    string          `json:"event_id"`
	Type       string          `json:"type"`
	BookingID  string          `json:"booking_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func NewEnvelope(eventType, bookingID string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		EventID:    uuid.NewString(),
		Type:       eventType,
		BookingID:  bookingID,
		OccurredAt: time.Now().UTC(),
		Payload:    raw,
	}, nil
}

func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func Unmarshal(data []byte) (Envelope, error) {
	var e Envelope
	err := json.Unmarshal(data, &e)
	return e, err
}
