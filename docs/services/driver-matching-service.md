# driver-matching-service

Owns the `drivers` table. Pure Kafka consumer — no HTTP API. Reacts to a booking
being requested by trying to reserve a driver, and independently reacts to payment
failing by releasing whatever driver it had reserved.

Module: `github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service`

## Domain model (`internal/domain/driver.go`)

```go
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
```

## Ports (`internal/domain/service.go`)

```go
var ErrNoDriverAvailable = errNoDriverAvailable{} // sentinel error

type Repository interface {
    FindAvailableDriver(ctx context.Context) (Driver, error) // or ErrNoDriverAvailable
    AssignDriver(ctx context.Context, driverID, bookingID string) error
    FindByBookingID(ctx context.Context, bookingID string) (Driver, error)
    ReleaseDriver(ctx context.Context, driverID string) error
}
```

## Methods

### `HandleBookingRequested(ctx, evt events.Envelope) error`

Reacts to `booking.requested`. Calls `FindAvailableDriver`:

- **`ErrNoDriverAvailable`**: publishes `driver.match_failed` (empty payload) and
  returns — no driver was ever reserved, so there's nothing to compensate later.
- **any other error**: propagated (infra failure — the message will be redelivered
  by Kafka's at-least-once retry, since the offset isn't committed on handler
  error; see [shared-module.md](../shared-module.md)).
- **success**: `AssignDriver(ctx, driver.ID, evt.BookingID)`, then publishes
  `driver.matched` with payload `{driver_id}`.

Driven out by two scenarios in
[features/match-driver.md](../features/match-driver.md) (the success and the
`ErrNoDriverAvailable` branch).

### `HandlePaymentFailed(ctx, evt events.Envelope) error`

Reacts to `payment.failed` — the **compensation**. Looks up the driver assigned to
`evt.BookingID` via `FindByBookingID`, `ReleaseDriver`s it (back to `AVAILABLE`,
assignment cleared), publishes `driver.released` (empty payload).
`booking-service` reacts to the same `payment.failed` event independently to
cancel the booking (see
[booking-service.md#handlepaymentfailedctx-evt-eventsenvelope-error](booking-service.md)) —
neither service knows the other exists; this is choreography. Driven out by
[features/release-driver.md](../features/release-driver.md).

## Kafka wiring (`internal/messaging/consumers.go`)

| Topic | Handler | Consumer group |
|---|---|---|
| `booking.requested` | `Service.HandleBookingRequested` | `driver-matching-service.booking.requested` |
| `payment.failed` | `Service.HandlePaymentFailed` | `driver-matching-service.payment.failed` |

Each topic gets its own consumer group ID for the same reason as
`booking-service` — see [infra/kafka.md](../infra/kafka.md).

## Postgres (`internal/repository/postgres.go`, `migrations/0001_create_drivers.sql`)

Database `driver_matching_service`, one table, seeded with three demo drivers on
first boot (there's no driver-onboarding API in this project):

```sql
CREATE TABLE IF NOT EXISTS drivers (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    status               TEXT NOT NULL,
    assigned_booking_id  TEXT NOT NULL DEFAULT ''
);

INSERT INTO drivers (id, name, status) VALUES
    ('driver-1', 'Alex Rivera', 'AVAILABLE'),
    ('driver-2', 'Sam Okafor', 'AVAILABLE'),
    ('driver-3', 'Priya Nair', 'AVAILABLE')
ON CONFLICT (id) DO NOTHING;
```

`PostgresRepository.AssignDriver` guards against a race between two bookings
grabbing the same driver with a conditional update, not a separate lock:

```sql
UPDATE drivers SET status = 'MATCHED', assigned_booking_id = $3
WHERE id = $1 AND status = 'AVAILABLE'
```
If `RowsAffected() == 0`, the driver wasn't actually available anymore (lost the
race) and an error is returned. This is a best-effort guard for the demo, not a
fully serialized `SELECT ... FOR UPDATE SKIP LOCKED` — see
[infra/postgres.md](../infra/postgres.md) for the tradeoff being made here.

## Configuration (env vars, see `cmd/main.go`)

| Var | Required | Purpose |
|---|---|---|
| `POSTGRES_DSN` | yes | `postgres://saga:saga@postgres:5432/driver_matching_service?sslmode=disable` |
| `KAFKA_BROKERS` | yes | comma-separated |
| `REDIS_ADDR` | yes | idempotency store |
| `IDEMPOTENCY_PREFIX` | no (default `driver-matching-service`) | Redis key prefix |

No `HTTP_ADDR` — this service exposes no HTTP port at all.

## Verified live

Watched `driver.matched` land on Kafka and the corresponding row in `drivers` flip
to `MATCHED` with the right `assigned_booking_id` after a `booking.requested` event;
watched a driver flip back to `AVAILABLE` with `driver.released` published after a
forced payment decline. See the root
[README.md](../../README.md#saga-flow).
