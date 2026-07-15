# payment-service

Owns the `payments` table. Pure Kafka consumer — no HTTP API. Reacts to a driver
being matched by charging the rider through a `Gateway` port.

Module: `github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service`

## Domain model (`internal/domain/payment.go`)

```go
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
```

## Ports (`internal/domain/service.go`)

```go
var ErrInsufficientFunds = errInsufficientFunds{} // sentinel error

type Gateway interface {
    Charge(ctx context.Context, bookingID string) error // or ErrInsufficientFunds
}

type Repository interface {
    Save(ctx context.Context, p Payment) error
}
```

`Gateway` is deliberately the *only* thing standing between this service and a real
payment processor — see [Simulated gateway](#simulated-gateway-internalgateway) below.

## Method

### `HandleDriverMatched(ctx, evt events.Envelope) error`

Reacts to `driver.matched`. Unlike most handlers in this saga, it actually needs
the event's `Payload` — it JSON-decodes it into `DriverMatchedPayload{DriverID
string}` to know which driver to record the payment against. Then:

1. `gateway.Charge(ctx, evt.BookingID)`.
2. **`ErrInsufficientFunds`**: `Save`s a `Payment{Status: StatusFailed}`, publishes
   `payment.failed` (empty payload) — this is the saga's compensation trigger,
   picked up independently by both `booking-service` and `driver-matching-service`.
3. **any other error**: propagated (infra failure, Kafka will redeliver).
4. **success**: `Save`s a `Payment{Status: StatusCompleted}`, publishes
   `payment.completed` (empty payload).

Driven out by two scenarios (success and decline) in
[features/charge-payment.md](../features/charge-payment.md).

## Simulated gateway (`internal/gateway/simulated.go`)

There is no real payment processor integrated in this project. `Simulated`
implements `Gateway` with one rule: decline any booking ID in a configurable set,
accept everything else.

```go
func NewSimulated(declineBookingIDs []string) *Simulated
func (g *Simulated) Charge(_ context.Context, bookingID string) error
```

Configured via the `DECLINE_BOOKING_IDS` env var (comma-separated booking IDs),
which is how the compensation path was exercised against the *live* stack (not
just BDD tests) during verification — start a payment-service instance with a
specific booking ID in that list and its charge will be declined deterministically.
See the root [README.md](../../README.md#running-it).

## Kafka wiring (`internal/messaging/consumers.go`)

Only one topic, so unlike the other services this one doesn't spawn a goroutine —
`messaging.Run` just calls `reader.Run` directly and blocks:

| Topic | Handler | Consumer group |
|---|---|---|
| `driver.matched` | `Service.HandleDriverMatched` | `payment-service` |

(No per-topic suffix needed here since there's only one topic — the bug described
in [infra/kafka.md](../infra/kafka.md) only bites when one group ID is shared
*across different topics*.)

## Postgres (`internal/repository/postgres.go`, `migrations/0001_create_payments.sql`)

Database `payment_service`, one table, insert-only (no updates — a payment record
is created once per `driver.matched` event and never changed):

```sql
CREATE TABLE IF NOT EXISTS payments (
    id          UUID PRIMARY KEY,
    booking_id  UUID NOT NULL,
    driver_id   TEXT NOT NULL,
    status      TEXT NOT NULL
);
```

## Configuration (env vars, see `cmd/main.go`)

| Var | Required | Purpose |
|---|---|---|
| `POSTGRES_DSN` | yes | `postgres://saga:saga@postgres:5432/payment_service?sslmode=disable` |
| `KAFKA_BROKERS` | yes | comma-separated |
| `REDIS_ADDR` | yes | idempotency store |
| `IDEMPOTENCY_PREFIX` | no (default `payment-service`) | Redis key prefix |
| `DECLINE_BOOKING_IDS` | no | comma-separated booking IDs the simulated gateway declines |

## Verified live

Confirmed both outcomes end-to-end: a normal booking gets a `COMPLETED` row in
`payments` and `payment.completed` published; a booking whose ID was passed via
`DECLINE_BOOKING_IDS` gets a `FAILED` row and `payment.failed` published, which
then cascaded into `booking-service` cancelling the booking and
`driver-matching-service` releasing the driver. See the root
[README.md](../../README.md#saga-flow).
