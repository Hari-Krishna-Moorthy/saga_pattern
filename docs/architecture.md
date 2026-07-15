# Architecture & the saga pattern

## Why a saga at all

A ride booking touches four separate concerns — booking state, driver availability,
payment, notifications — each with its own data and its own service. A single
booking can't be one local database transaction across four services, so instead
the system runs it as a **saga**: a sequence of local transactions, each in its own
service, coordinated by events. If a later step fails, earlier steps are undone by
**compensating transactions** rather than a database rollback.

## Choreography, not orchestration

There are two ways to build a saga:

- **Orchestration**: one service (a "saga orchestrator") owns the whole workflow,
  telling each participant what to do next and calling their compensations by name.
- **Choreography**: there is no coordinator. Every service publishes events when its
  own local transaction completes, and every service that cares about an event
  subscribes to it and reacts. The workflow's shape lives implicitly in the set of
  publishers and subscribers, not in one file.

This project uses **choreography**, over Kafka. Each service:

1. Owns exactly one Postgres database (or nothing, for notification-service).
2. Reacts to the Kafka topics it cares about by running its own local transaction.
3. Publishes an event describing what happened.
4. Knows nothing about which other services exist or what they'll do next.

The tradeoff, visible directly in this codebase: when payment fails
(`payment.failed`), **two different services** — `booking-service` and
`driver-matching-service` — each independently subscribe to that one event and run
their own compensation (cancel the booking; release the driver). There's no single
place you can read to see "the compensation" — you have to know which services
subscribe to which topics. That's the cost of choreography; the benefit is that no
service needs to know about, or call into, any other service.

## Service responsibility table

| Service | Owns (Postgres) | Reacts to (consumes) | Publishes |
|---|---|---|---|
| `booking-service` | `bookings` | `payment.completed`, `driver.match_failed`, `payment.failed` | `booking.requested`, `ride.confirmed`, `booking.cancelled` |
| `driver-matching-service` | `drivers` | `booking.requested`, `payment.failed` | `driver.matched`, `driver.match_failed`, `driver.released` |
| `payment-service` | `payments` | `driver.matched` | `payment.completed`, `payment.failed` |
| `notification-service` | — (stateless) | `ride.confirmed`, `booking.cancelled` | — (side effect only: logs a notification) |

## Event flow: happy path

```
rider
  │ POST /bookings
  ▼
booking-service           saves Booking{status: REQUESTED}
  │ publishes booking.requested
  ▼
driver-matching-service   finds an AVAILABLE driver, marks it MATCHED
  │ publishes driver.matched {driver_id}
  ▼
payment-service           charges the rider via the Gateway port
  │ publishes payment.completed
  ▼
booking-service           updates Booking.status = CONFIRMED
  │ publishes ride.confirmed
  ▼
notification-service      sends "Your ride is confirmed!"
```

## Event flow: compensation — no driver available

```
booking-service            publishes booking.requested
driver-matching-service    no AVAILABLE driver found
  │ publishes driver.match_failed
  ▼
booking-service            Booking.status = CANCELLED, reason "no driver available"
  │ publishes booking.cancelled {reason}
  ▼
notification-service       sends "Your ride was cancelled: no driver available"
```

No driver was ever reserved in this path, so there's nothing for
`driver-matching-service` to release — only `booking-service` reacts to
`driver.match_failed`.

## Event flow: compensation — payment declined

This is the interesting one: a driver *was* already reserved, so undoing the saga
means unwinding **two** services' local state, and each does so independently by
reacting to the same event:

```
payment-service             gateway declines the charge
  │ publishes payment.failed
  ├──────────────────────────────────────────────┐
  ▼                                               ▼
booking-service                        driver-matching-service
  Booking.status = CANCELLED              finds the driver assigned to
  reason "payment failed"                 this booking, sets it back to
  publishes booking.cancelled             AVAILABLE, clears the assignment
                                           publishes driver.released
  │
  ▼
notification-service
  sends "Your ride was cancelled: payment failed"
```

`driver.released` has no further subscriber in this project — it exists so a future
service (e.g. driver-facing notifications, or an analytics consumer) could react to
it without any existing service needing to change.

## Idempotency, not exactly-once

Kafka only guarantees **at-least-once** delivery — a consumer can see the same
message again after a crash-and-restart before it committed an offset. Every
consumer in this project therefore checks a Redis-backed idempotency guard
(`shared/idempotency`) keyed by the event's UUID before applying its saga reaction,
so a redelivered `payment.completed` doesn't confirm a booking twice. See
[shared-module.md](shared-module.md#sharedidempotency--dedup-guard) for the exact mechanism.

## Database-per-service

Each service that needs persistence owns its own Postgres **database** (not just a
schema) — `booking_service`, `driver_matching_service`, `payment_service`. No
service ever queries another's tables. The only way services observe each other's
state is through the events they publish. This is what makes choreography
consistent with the "independently deployable microservice" premise: a schema
change in one service's database can never silently break another service, because
there's no shared schema to change.

See also: [tech-stack.md](tech-stack.md) for what each of Kafka/Postgres/Redis/Go is
doing here, and [bdd-workflow.md](bdd-workflow.md) for how this logic was built and
tested.
