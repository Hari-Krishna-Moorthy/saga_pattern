# booking-service

Owns the `bookings` table. The only service in the saga with an HTTP API — it's
the rider-facing entry point. Everything else it does is reacting to events.

Module: `github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service`

## Domain model (`internal/domain/booking.go`)

```go
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
```

## Ports (`internal/domain/service.go`)

```go
type Repository interface {
    Save(ctx context.Context, b Booking) error
    FindByID(ctx context.Context, id string) (Booking, error)
    Update(ctx context.Context, b Booking) error
}
```

`Service` holds a `Repository` and a `kafka.Publisher` (from `shared/kafka`) — see
[shared-module.md](../shared-module.md).

## Methods

### `RequestBooking(ctx, riderID, pickup, dropoff string) (Booking, error)`

Called directly by the HTTP handler (not by a Kafka consumer — this is the saga's
starting point). Builds a new `Booking{Status: StatusRequested}` with a fresh UUID,
`Save`s it, then publishes `booking.requested` with payload
`{rider_id, pickup, dropoff}`. Driven out by
[features/request-booking.md](../features/request-booking.md).

### `HandlePaymentCompleted(ctx, evt events.Envelope) error`

Reacts to `payment.completed`. Loads the booking by `evt.BookingID`, sets
`Status = CONFIRMED`, `Update`s it, publishes `ride.confirmed` (empty payload — the
booking ID in the envelope is all any subscriber needs). This is the saga's
successful terminal state. Driven out by
[features/confirm-booking.md](../features/confirm-booking.md).

### `HandleDriverMatchFailed(ctx, evt events.Envelope) error`

Reacts to `driver.match_failed`. Delegates to `cancelBooking(ctx, evt.BookingID,
"no driver available")`. Driven out by
[features/cancel-booking.md](../features/cancel-booking.md).

### `HandlePaymentFailed(ctx, evt events.Envelope) error`

Reacts to `payment.failed` — this is the **compensation** trigger. Delegates to
`cancelBooking(ctx, evt.BookingID, "payment failed")`.
`driver-matching-service` reacts to this *same* event independently to release the
driver it had reserved (see
[driver-matching-service.md](driver-matching-service.md#handlepaymentfailed))
— neither service calls the other; both just subscribe to `payment.failed`. Driven
out by [features/cancel-booking.md](../features/cancel-booking.md).

### `cancelBooking(ctx, bookingID, reason string) error` (private)

Shared by the two `Handle*Failed` methods above: loads the booking, sets
`Status = CANCELLED` and `CancelReason = reason`, `Update`s it, publishes
`booking.cancelled` with payload `{reason}` — this is why
`notification-service` can say *why* a ride was cancelled without knowing anything
about bookings, drivers, or payments itself.

## HTTP API (`internal/httpapi/handler.go`)

The only HTTP surface in the whole saga.

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/bookings` | `{"rider_id","pickup","dropoff"}` | `201` + the created `Booking` as JSON |
| `GET` | `/bookings/{id}` | — | `200` + the `Booking` as JSON, or `404` |

`POST /bookings` calls `Service.RequestBooking` directly (not through Kafka — HTTP
requests are synchronous by nature). `GET /bookings/{id}` calls
`Repository.FindByID` directly (bypassing the `Service` — reads don't need saga
logic, just a lookup) via a `bookingFinder` interface the `PostgresRepository`
already satisfies.

## Kafka wiring (`internal/messaging/consumers.go`)

One goroutine per subscribed topic, each with its **own** consumer group ID
(`booking-service.<topic>` — see [infra/kafka.md](../infra/kafka.md) for why a
shared group ID across topics breaks):

| Topic | Handler |
|---|---|
| `payment.completed` | `Service.HandlePaymentCompleted` |
| `driver.match_failed` | `Service.HandleDriverMatchFailed` |
| `payment.failed` | `Service.HandlePaymentFailed` |

Every message is idempotency-checked (Redis, prefix `booking-service`) before the
handler runs — see [shared-module.md#idempotency](../shared-module.md#sharedidempotency--dedup-guard).

## Postgres (`internal/repository/postgres.go`, `migrations/0001_create_bookings.sql`)

Database `booking_service`, one table:

```sql
CREATE TABLE IF NOT EXISTS bookings (
    id             UUID PRIMARY KEY,
    rider_id       TEXT NOT NULL,
    pickup         TEXT NOT NULL,
    dropoff        TEXT NOT NULL,
    status         TEXT NOT NULL,
    cancel_reason  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);
```

`PostgresRepository` implements `Repository` with plain `pgxpool` queries — no ORM.
`Update` checks `RowsAffected() == 0` and returns an error if the booking didn't
exist, rather than silently no-op'ing.

## Configuration (env vars, see `cmd/main.go`)

| Var | Required | Purpose |
|---|---|---|
| `POSTGRES_DSN` | yes | e.g. `postgres://saga:saga@postgres:5432/booking_service?sslmode=disable` |
| `KAFKA_BROKERS` | yes | comma-separated, e.g. `kafka:9092` |
| `REDIS_ADDR` | yes | e.g. `redis:6379` |
| `HTTP_ADDR` | no (default `:8080`) | HTTP listen address |
| `IDEMPOTENCY_PREFIX` | no (default `booking-service`) | Redis key prefix |

## Verified live

`POST /bookings` → row appears in Postgres with `status=REQUESTED` → `booking.requested`
observed on the Kafka topic → (once driver-matching/payment services react)
`GET /bookings/{id}` eventually returns `CONFIRMED` or `CANCELLED` with the correct
reason. See the root [README.md](../../README.md#saga-flow) for the full loop.
