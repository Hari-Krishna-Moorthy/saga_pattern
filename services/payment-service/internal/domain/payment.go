package domain

type Status string

const (
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

type Payment struct {
	ID        string
	BookingID string
	DriverID  string
	Status    Status
}
