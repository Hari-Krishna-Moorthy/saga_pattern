# notification-service

Stateless — no Postgres database, no HTTP API. Reacts to the saga's two terminal
outcomes and sends a notification. The simplest service in the project: nothing
downstream reacts to anything it does, so it never publishes an event.

Module: `github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/notification-service`

## Port (`internal/domain/service.go`)

```go
type Notifier interface {
    Send(ctx context.Context, bookingID, message string) error
}
```

Just one method — this service does nothing else, so it needs nothing else.

## Methods

### `HandleRideConfirmed(ctx, evt events.Envelope) error`

Reacts to `ride.confirmed` — the saga's successful terminal state. Sends a fixed
message: `"Your ride is confirmed!"`. Driven out by
[features/notify-rider.md](../features/notify-rider.md).

### `HandleBookingCancelled(ctx, evt events.Envelope) error`

Reacts to `booking.cancelled` — the saga's compensated terminal state. Unlike
`HandleRideConfirmed`, this one reads the event `Payload`:

```go
type bookingCancelledPayload struct {
    Reason string `json:"reason"`
}
```

decoded best-effort (`_ = json.Unmarshal(...)` — a malformed/missing payload just
falls back to a generic message rather than failing the whole handler), then sends
`"Your ride was cancelled: " + reason`, or `"Your ride was cancelled."` if no
reason was present. This is why `booking-service`'s `cancelBooking` publishes
`booking.cancelled` with a `{reason}` payload instead of an empty one — it exists
specifically so this service can surface *why* to the rider. Driven out by
[features/notify-rider.md](../features/notify-rider.md).

## Log notifier (`internal/notifier/log.go`)

There is no real push/SMS/email provider integrated. `Log` implements `Notifier`
by writing one line to stdout:

```go
func (l *Log) Send(_ context.Context, bookingID, message string) error {
    log.Printf("notify booking=%s: %s", bookingID, message)
    return nil
}
```

`docker compose logs -f notification-service` is how you watch the saga's outcomes
happen live — every booking's terminal state (confirmed or cancelled, with reason)
shows up here.

## Kafka wiring (`internal/messaging/consumers.go`)

| Topic | Handler | Consumer group |
|---|---|---|
| `ride.confirmed` | `Service.HandleRideConfirmed` | `notification-service.ride.confirmed` |
| `booking.cancelled` | `Service.HandleBookingCancelled` | `notification-service.booking.cancelled` |

Two topics, so — same as `booking-service` and `driver-matching-service` — each
gets its own consumer group ID. See [infra/kafka.md](../infra/kafka.md).

## Configuration (env vars, see `cmd/main.go`)

| Var | Required | Purpose |
|---|---|---|
| `KAFKA_BROKERS` | yes | comma-separated |
| `REDIS_ADDR` | yes | idempotency store |
| `IDEMPOTENCY_PREFIX` | no (default `notification-service`) | Redis key prefix |

No `POSTGRES_DSN` — this is the one service in the saga with nothing to persist.

## Verified live

Watched both log lines appear for real bookings run through the full stack:

```
notify booking=<id>: Your ride is confirmed!
notify booking=<id>: Your ride was cancelled: payment failed
notify booking=<id>: Your ride was cancelled: no driver available
```
