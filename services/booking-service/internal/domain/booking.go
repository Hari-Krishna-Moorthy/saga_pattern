package domain

import "time"

type Status string

const (
	StatusRequested Status = "REQUESTED"
	StatusConfirmed Status = "CONFIRMED"
	StatusCancelled Status = "CANCELLED"
)

type Booking struct {
	ID           string
	RiderID      string
	Pickup       string
	Dropoff      string
	Status       Status
	CancelReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
