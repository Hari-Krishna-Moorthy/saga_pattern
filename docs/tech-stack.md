# Technology stack

## Language & module layout

**Go 1.25** (services declare `go 1.25.0` / `go 1.24`; the toolchain auto-upgrades
via `GOTOOLCHAIN=auto`, which is how a machine with Go 1.23 installed transparently
built this project — see `go.work`).

The repo is a single [**Go workspace**](https://go.dev/ref/mod#workspaces)
(`go.work` at the repo root) containing five independent Go modules:

```
go.work
├── shared                              (github.com/.../shared)
├── services/booking-service             (github.com/.../services/booking-service)
├── services/driver-matching-service
├── services/payment-service
└── services/notification-service
```

Why a workspace instead of one module: each service is meant to be an
independently deployable microservice with its own `go.mod`/`go.sum`, so a
dependency bump in one service can't force a rebuild of the others. But they all
need to import `shared` (the event envelope, Kafka helpers, idempotency checker)
without publishing it anywhere. `go.work`'s `use` directives resolve that import
locally across the five modules with no `replace` directives and no version
tags — `go build`/`go test`/`go vet` all just work from any of the module
directories, or via `go.work` from the repo root for multi-module patterns like
`go list -m all`.

One consequence, hit during setup: `go mod tidy` run inside a single service module
tries to resolve the `shared` import over the network (since `shared` isn't a real
published module) and fails. `go build`/`go vet`/`go test` don't have this problem —
they resolve workspace members directly. So `go.mod`/`go.sum` in each service were
edited by hand / via `go get <third-party package>` rather than a blanket
`go mod tidy`.

## Kafka — event backbone (choreography saga transport)

**Broker**: `apache/kafka:3.7.0`, run in **KRaft mode** (`docker-compose.yml`) — no
ZooKeeper. One broker acts as both `broker` and `controller` role
(`KAFKA_PROCESS_ROLES: broker,controller`), which is enough for a demo/dev stack but
not how you'd run Kafka in production (no replication, no fault tolerance).
`KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"` means services never need a topic-creation
step — the first produce or consume attempt creates the topic.

**Client library**: [`segmentio/kafka-go`](https://github.com/segmentio/kafka-go) —
a pure-Go client (no cgo, no bundled librdkafka), which keeps the Docker images
small and the cross-compilation simple (`CGO_ENABLED=0` in every Dockerfile).
Wrapped in `shared/kafka` (see [shared-module.md](shared-module.md)) so services
depend on small `Publisher`/`Handler` interfaces rather than the kafka-go API
directly — that's what lets BDD tests use in-memory fakes instead of a real broker.

**Gotcha found and fixed**: Kafka's consumer-group protocol expects every member of
a group to subscribe to the *same* topic set. Two services initially reused one
group ID across multiple different-topic readers, which silently broke partition
assignment (0 partitions assigned, no errors logged, no messages ever delivered).
Fixed by suffixing the group ID with the topic name. Full writeup:
[infra/kafka.md](infra/kafka.md#the-consumer-group-bug).

## Postgres — durable state, one database per service

**Image**: `postgres:16-alpine`. One container, three databases
(`booking_service`, `driver_matching_service`, `payment_service`), created by
`deploy/postgres/init-databases.sh` on first boot — see
[infra/postgres.md](infra/postgres.md). `notification-service` is stateless and has
no database.

**Client library**: [`jackc/pgx/v5`](https://github.com/jackc/pgx), specifically
`pgxpool` for a connection pool. Chosen over `database/sql` + `lib/pq` because pgx
is the actively maintained, faster driver for Postgres specifically (no generic
`database/sql` abstraction overhead), and its native (non-`database/sql`) API is
what each service's `internal/repository` package uses directly.

**Migrations**: no migration framework (no `golang-migrate`, no `goose`). Each
service embeds its own `.sql` files via `go:embed` (`migrations/migrations.go` in
each service) and runs them with plain `pool.Exec` on startup, sorted by filename.
This keeps the binary self-contained — no separate migration step or tool needed to
deploy a service — at the cost of no rollback support, which is an acceptable
tradeoff for a demo with append-only, `CREATE TABLE IF NOT EXISTS`-style migrations.

## Redis — idempotency only

**Image**: `redis:7-alpine`. **Client library**:
[`redis/go-redis/v9`](https://github.com/redis/go-redis). Used for exactly one
thing across all four services: `SETNX <prefix>:<event-id> 1 EX 24h` before
applying a saga reaction, so an at-least-once Kafka redelivery can't double-apply a
side effect (confirm a booking twice, charge twice, etc.). It is **not** used as a
cache, a session store, or a lock — see
[shared-module.md#idempotency](shared-module.md#sharedidempotency--dedup-guard) and
[infra/redis.md](infra/redis.md).

## Godog — BDD testing

[`cucumber/godog`](https://github.com/cucumber/godog), the official Go
implementation of Cucumber. Every saga reaction in every service was built by
writing a `.feature` file (Gherkin: Given/When/Then) first, then step definitions
that call into the service's domain layer, then just enough production code to make
it pass. Full explanation, including why this doesn't need Docker to run:
[bdd-workflow.md](bdd-workflow.md).

## Docker & Docker Compose

Each service has its own multi-stage `Dockerfile`
(`golang:1.25-alpine` build stage → `alpine:3.20` runtime stage, `CGO_ENABLED=0`,
final image is just the static binary + CA certs). `docker-compose.yml` at the repo
root wires all seven containers (Postgres, Redis, Kafka, the four services)
together with healthchecks and `depends_on: condition: service_healthy` so services
don't start racing infrastructure that isn't ready yet. Details:
[infra/docker-compose.md](infra/docker-compose.md).

## What's deliberately *not* here

- **No API gateway** — only `booking-service` has an HTTP API; the others are pure
  Kafka consumers, reachable only through events, which is normal for choreography
  participants that don't need a synchronous entry point.
- **No schema registry** — events are plain JSON (`shared/events.Envelope`), not
  Avro/Protobuf. Simpler for a demo; a real system at scale would want schema
  evolution guarantees a registry provides.
- **No distributed tracing** — there's no correlation ID threaded through the saga
  beyond the booking ID embedded in every event envelope. Good enough to follow a
  booking through logs by grepping its ID; not good enough for production-grade
  observability across services.
