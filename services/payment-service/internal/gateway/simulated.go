// Package gateway provides a stand-in payment processor for the demo:
// there's no real payment provider integration here, just a deterministic
// rule so the insufficient-funds compensation path can be exercised on the
// live stack, not just in tests.
package gateway

import (
	"context"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service/internal/domain"
)

type Simulated struct {
	declineBookingIDs map[string]bool
}

// NewSimulated builds a gateway that declines charges for the given
// booking IDs (for demo purposes) and accepts everything else.
func NewSimulated(declineBookingIDs []string) *Simulated {
	set := make(map[string]bool, len(declineBookingIDs))
	for _, id := range declineBookingIDs {
		if id != "" {
			set[id] = true
		}
	}
	return &Simulated{declineBookingIDs: set}
}

func (g *Simulated) Charge(_ context.Context, bookingID string) error {
	if g.declineBookingIDs[bookingID] {
		return domain.ErrInsufficientFunds
	}
	return nil
}
