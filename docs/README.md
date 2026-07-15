# ride-booking-saga documentation

This folder documents the project in depth: the saga architecture, the technology
choices, the shared library, every service's methods, every BDD feature/scenario,
and the infrastructure pieces (Kafka, Postgres, Redis, Docker Compose).

Start here, then drill into whichever area you need:

## Concepts

- [Architecture & the saga pattern](architecture.md) — choreography vs. orchestration,
  the full event flow (happy path + both compensations), service responsibility table.
- [Technology stack](tech-stack.md) — every library used and why, with versions.
- [BDD workflow](bdd-workflow.md) — how Gherkin/Godog scenarios drove every piece of
  saga logic, and the ports-and-adapters shape that makes that possible.
- [Shared module](shared-module.md) — the `shared/` Go module's public API
  (event envelope, Kafka publisher/consumer, Redis idempotency checker), used by
  all four services.

## Services

One doc per service: what it owns, its domain methods, its HTTP API (if any), its
Postgres schema, and the Kafka topics it consumes/publishes.

- [booking-service](services/booking-service.md)
- [driver-matching-service](services/driver-matching-service.md)
- [payment-service](services/payment-service.md)
- [notification-service](services/notification-service.md)

## Features (BDD scenarios)

One doc per `.feature` file — every scenario's Given/When/Then, the production
method it exercises, and where it sits in the saga.

- [request-booking](features/request-booking.md) — `booking-service`
- [confirm-booking](features/confirm-booking.md) — `booking-service`
- [cancel-booking](features/cancel-booking.md) — `booking-service`
- [match-driver](features/match-driver.md) — `driver-matching-service`
- [release-driver](features/release-driver.md) — `driver-matching-service`
- [charge-payment](features/charge-payment.md) — `payment-service`
- [notify-rider](features/notify-rider.md) — `notification-service`

## Infrastructure

- [Docker Compose](infra/docker-compose.md) — how the seven containers fit together.
- [Postgres](infra/postgres.md) — database-per-service, embedded migrations.
- [Kafka](infra/kafka.md) — KRaft single-node setup, topics, the consumer-group bug
  found during verification and its fix.
- [Redis](infra/redis.md) — idempotency-only usage, key scheme.
