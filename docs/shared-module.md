# Shared module (`shared/`)

Go module `github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared`, imported by
all four services via the `go.work` workspace (see
[tech-stack.md](tech-stack.md#language--module-layout)). Three packages, each with
a small, deliberately narrow public API.

## `shared/events` — the wire format

### `Envelope` (`envelope.go`)

Every event published to Kafka, by every service, is this struct, JSON-encoded:

```go
type Envelope struct {
    EventID    string          `json:"event_id"`
    Type       string          `json:"type"`
    BookingID  string          `json:"booking_id"`
    OccurredAt time.Time       `json:"occurred_at"`
    Payload    json.RawMessage `json:"payload"`
}
```

- `EventID` — a fresh UUID (`google/uuid`) generated per event. This is the
  idempotency key (see [idempotency](#sharedidempotency--dedup-guard) below) — it
  identifies *this specific publish*, not the booking or the event type.
- `BookingID` — every event in this saga is scoped to one booking, so it's a
  top-level field (not buried in `Payload`) — consumers key off it directly (e.g.
  `evt.BookingID`) without unmarshalling the payload.
- `Payload` — left as `json.RawMessage` (undecoded bytes) so the envelope itself
  never needs to know the shape of any particular event's data. Each consumer
  decodes it into its own local struct only if it actually needs fields from it
  (see `payment-service`'s `DriverMatchedPayload` for an example — most consumers
  in this saga don't need the payload at all, since the booking ID is enough).

**Functions:**

```go
func NewEnvelope(eventType, bookingID string, payload any) (Envelope, error)
```
Builds an envelope: generates the `EventID`, stamps `OccurredAt` (UTC), and
JSON-marshals `payload` into `Payload`. This is what every `Handle*`/`Request*`
method in every service's domain layer calls before publishing.

```go
func (e Envelope) Marshal() ([]byte, error)
func Unmarshal(data []byte) (Envelope, error)
```
Marshal/unmarshal the envelope itself (not the payload) to/from JSON bytes — used
by `kafka.Writer.Publish` and `kafka.Reader.Run` respectively.

### `Topics` (`topics.go`)

Every Kafka topic name used in the saga, as typed constants, plus an `AllTopics`
slice:

```go
TopicBookingRequested   = "booking.requested"
TopicBookingCancelled   = "booking.cancelled"
TopicDriverMatched      = "driver.matched"
TopicDriverMatchFailed  = "driver.match_failed"
TopicDriverReleased     = "driver.released"   // compensation
TopicPaymentCompleted   = "payment.completed"
TopicPaymentFailed      = "payment.failed"
TopicRideConfirmed      = "ride.confirmed"
```

Centralizing these as constants (rather than string literals scattered across
services) is what makes it possible to grep the whole saga's topology: every
`Publish(ctx, events.TopicX, ...)` and every `handlers[events.TopicX]` map entry
across all four services references the same symbol.

## `shared/kafka` — publish/subscribe, behind small interfaces

### `Publisher` interface (`publisher.go`)

```go
type Publisher interface {
    Publish(ctx context.Context, topic string, evt events.Envelope) error
}
```

This is the **port** every service's domain layer depends on — never the concrete
kafka-go client. That's what lets BDD tests substitute an in-memory `fakePublisher`
(see [bdd-workflow.md](bdd-workflow.md)) with zero test-only branches in production
code.

### `Writer` — the real implementation

```go
func NewWriter(brokers []string) *Writer
func (w *Writer) Publish(ctx context.Context, topic string, evt events.Envelope) error
func (w *Writer) Close() error
```

Lazily creates one `kafka-go` `Writer` per topic on first publish (`w.writers` map),
each configured with `AllowAutoTopicCreation: true` and `LeastBytes` load balancing.
Uses `evt.BookingID` as the Kafka message key, so all events for one booking land on
the same partition and are delivered in order relative to each other.

### `Handler` and `Reader` (`consumer.go`)

```go
type Handler func(ctx context.Context, evt events.Envelope) error

func NewReader(brokers []string, topic, groupID string) *Reader
func (r *Reader) Run(ctx context.Context, handle Handler) error
func (r *Reader) Close() error
```

`Run` is a blocking loop: `FetchMessage` → unmarshal the envelope → call `handle` →
`CommitMessages` (manual offset commit, only after `handle` succeeds — see below) →
repeat, until `ctx` is cancelled.

Two failure paths, both intentional:

- **Malformed message** (fails `events.Unmarshal`): logged and the offset is
  committed anyway — a message that will never parse would otherwise block the
  partition forever on redelivery.
- **Handler error** (`handle` returns non-nil): logged, offset is **not**
  committed, loop continues to the next `FetchMessage` call. Since the offset
  wasn't committed, kafka-go redelivers this message on the next fetch, giving
  at-least-once retry semantics for transient failures (e.g. Postgres briefly
  unavailable).

Every service wires this the same way in its `internal/messaging/consumers.go`:
one goroutine per subscribed topic, each with its own `Reader`, each wrapping
`handle` in the idempotency check before calling into the domain `Service`. See
each service's doc for its exact topic → handler map.

## `shared/idempotency` — dedup guard

```go
type Checker interface {
    AlreadyProcessed(ctx context.Context, eventID string) (bool, error)
}
```

Again, a small interface — the port every service's `messaging.Run` depends on.

```go
func NewRedisChecker(client *redis.Client, keyPrefix string) *RedisChecker
func (c *RedisChecker) AlreadyProcessed(ctx context.Context, eventID string) (bool, error)
```

`AlreadyProcessed` does one atomic Redis call:

```
SETNX <keyPrefix>:<eventID> 1 EX 24h
```

`SETNX` ("set if not exists") returns whether the key was newly set. If it *was*
newly set (this is the first time we've seen this event ID), `AlreadyProcessed`
returns `false` — go ahead and process it. If the key already existed (we've seen
this exact event before, most likely a Kafka redelivery), it returns `true` — skip
it. The 24-hour TTL bounds Redis memory growth; a booking's saga always completes
in seconds, so 24 hours is a large safety margin, not a tight one.

Each service passes its own `keyPrefix` (`IDEMPOTENCY_PREFIX` env var, defaulting
to the service name) so the four services' idempotency keys never collide in the
one shared Redis instance — see [infra/redis.md](infra/redis.md).

Every consumer's wiring follows the same shape (shown here from
`booking-service/internal/messaging/consumers.go`, identical pattern in the other
three services):

```go
err := reader.Run(ctx, func(ctx context.Context, evt events.Envelope) error {
    seen, err := idem.AlreadyProcessed(ctx, evt.EventID)
    if err != nil {
        return err
    }
    if seen {
        log.Printf("skipping already-processed event %s on %s", evt.EventID, topic)
        return nil
    }
    return handle(ctx, evt) // the real domain method, e.g. svc.HandlePaymentCompleted
})
```
