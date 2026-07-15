# ride-booking-saga

A ride-booking system built as four independent Go microservices that coordinate a
distributed transaction via the **saga pattern (choreography style)**: each service
publishes domain events to Kafka and reacts to events published by the others. There
is no central orchestrator.

Built test-first: every saga reaction is driven out via a Gherkin/Godog BDD scenario
before the production wiring goes in (see [Development workflow](#development-workflow)).

## Services

| Service | Owns | Reacts to | Publishes |
|---|---|---|---|
| `booking-service` | `bookings` (Postgres) | `payment.completed`, `driver.match_failed`, `payment.failed` | `booking.requested`, `ride.confirmed`, `booking.cancelled` |
| `driver-matching-service` | `drivers` (Postgres) | `booking.requested`, `payment.failed` | `driver.matched`, `driver.match_failed`, `driver.released` |
| `payment-service` | `payments` (Postgres) | `driver.matched` | `payment.completed`, `payment.failed` |
| `notification-service` | — | `ride.confirmed`, `booking.cancelled` | — (logs a notification) |

Each service owns its own Postgres database (database-per-service). Redis is shared
but namespaced per service, used only to make Kafka's at-least-once delivery
idempotent (`SETNX` on event ID before applying a saga reaction).

## Saga flow

**Happy path:**

```
rider -> POST /bookings -> booking-service
  booking.requested
    -> driver-matching-service matches a driver
       driver.matched
         -> payment-service charges the rider
            payment.completed
              -> booking-service confirms the booking
                 ride.confirmed
                   -> notification-service notifies the rider
```

**Compensation (payment declined after a driver was matched):**

```
payment.failed
  -> booking-service cancels the booking, publishes booking.cancelled
  -> driver-matching-service (independently) releases the driver, publishes driver.released
  -> notification-service notifies the rider of the cancellation
```

**Compensation (no driver available):**

```
driver.match_failed -> booking-service cancels the booking -> booking.cancelled -> notification-service notifies the rider
```

Because this is choreography, not orchestration, `booking-service` and
`driver-matching-service` each react to `payment.failed` independently — there's no
single place that "runs" the compensation, which is the point of the pattern (and the
tradeoff: the saga's shape lives implicitly in the set of consumers, not in one file).

## Running it

Requires Docker.

```
docker compose up -d --build
```

This starts Postgres (with `booking_service`, `driver_matching_service`, and
`payment_service` databases, one per service), Redis, a single-node Kafka broker
(KRaft mode), and all four services. `driver-matching-service` seeds three demo
drivers on first boot (there's no driver-onboarding API in this demo).

**On a fully cold start** (first boot, or right after `docker compose down -v`),
Kafka's group coordinator can take up to a minute to stabilize all five consumer
groups on constrained hardware — bookings created in that window will sit at
`REQUESTED` until the consumers catch up, then resolve automatically (nothing is
lost; Kafka retains the events and delivers them once each group's rebalance
settles). `docker compose logs -f` across the services will show it happening. This
isn't saga-specific — it's Kafka group-join latency on a cold broker.

Create a booking:

```
curl -X POST http://localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -d '{"rider_id":"rider-1","pickup":"123 Main St","dropoff":"456 Oak Ave"}'
```

Check its status (watch it move REQUESTED -> CONFIRMED, or REQUESTED -> CANCELLED if
no driver is free — only 3 demo drivers exist):

```
curl http://localhost:8080/bookings/<id>
```

Watch the saga happen: `docker compose logs -f notification-service` shows the
terminal outcome of every booking.

To force the compensation path against the live stack, run `payment-service` with
`DECLINE_BOOKING_IDS=<booking-id>` set (comma-separated) — it's a simulated gateway,
not a real payment processor.

## Development workflow

Every saga reaction was built BDD-first: a Gherkin scenario in each service's
`features/` directory, run via [Godog](https://github.com/cucumber/godog), against
an in-memory fake of that service's Repository/Publisher/Gateway ports — no Docker
required to run these. Real Postgres/Kafka/Redis adapters implement the same
interfaces and are only wired together in each service's `cmd/main.go`.

Run one service's BDD suite:

```
cd services/booking-service && go test ./... -v
```

Run everything:

```
for svc in shared services/booking-service services/driver-matching-service services/payment-service services/notification-service; do
  (cd $svc && go build ./... && go vet ./... && go test ./...)
done
```

The repo is a single [Go workspace](https://go.dev/ref/mod#workspaces) (`go.work`)
so the services can share the `shared` module (event envelope, Kafka
producer/consumer, Redis idempotency helper) without it being published anywhere.

## Layout

```
shared/                  event envelope, Kafka pub/sub, Redis idempotency (used by all services)
services/<name>/
  features/              Gherkin scenarios
  internal/domain/       saga reaction logic + ports (Repository, Publisher, ...) + BDD step defs
  internal/repository/   Postgres adapter
  internal/messaging/    Kafka consumer wiring
  internal/httpapi/      HTTP API (booking-service only)
  migrations/            embedded SQL schema, applied on startup
  cmd/main.go            wiring
  Dockerfile
docker-compose.yml
deploy/postgres/         database-per-service init script
```
