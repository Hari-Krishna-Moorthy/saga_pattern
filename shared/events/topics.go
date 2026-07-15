package events

// Kafka topics used across the ride-booking saga (choreography style).
// Each service publishes its own domain events and subscribes to the
// events it needs to react to; there is no central orchestrator.
const (
	TopicBookingRequested = "booking.requested"
	TopicBookingCancelled = "booking.cancelled"

	TopicDriverMatched     = "driver.matched"
	TopicDriverMatchFailed = "driver.match_failed"
	TopicDriverReleased    = "driver.released" // compensation

	TopicPaymentCompleted = "payment.completed"
	TopicPaymentFailed    = "payment.failed"

	TopicRideConfirmed = "ride.confirmed"
)

// AllTopics is used to pre-create topics on startup/in docker-compose.
var AllTopics = []string{
	TopicBookingRequested,
	TopicBookingCancelled,
	TopicDriverMatched,
	TopicDriverMatchFailed,
	TopicDriverReleased,
	TopicPaymentCompleted,
	TopicPaymentFailed,
	TopicRideConfirmed,
}
