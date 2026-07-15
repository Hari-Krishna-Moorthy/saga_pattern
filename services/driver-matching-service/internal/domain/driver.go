package domain

type DriverStatus string

const (
	DriverAvailable DriverStatus = "AVAILABLE"
	DriverMatched   DriverStatus = "MATCHED"
)

type Driver struct {
	ID                string
	Name              string
	Status            DriverStatus
	AssignedBookingID string
}
