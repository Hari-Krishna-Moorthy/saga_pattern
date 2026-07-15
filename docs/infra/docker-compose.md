# Docker Compose

`docker-compose.yml` at the repo root wires seven containers: three pieces of
infrastructure (Postgres, Redis, Kafka) and the four services.

## Shared environment via YAML anchors

```yaml
x-service-env: &service-env
  KAFKA_BROKERS: kafka:9092
  REDIS_ADDR: redis:6379
```

Every service's `environment:` block starts with `<<: *service-env` and then adds
its own service-specific variables (its `POSTGRES_DSN`, its
`IDEMPOTENCY_PREFIX`, and — for `booking-service` only — `HTTP_ADDR`). This avoids
repeating `KAFKA_BROKERS`/`REDIS_ADDR` four times with the risk of one of them
drifting.

## Startup ordering: `depends_on` + healthchecks

Each of the three infra containers has a `healthcheck`, and every service's
`depends_on` uses `condition: service_healthy`, not just "container started":

```yaml
depends_on:
  postgres:
    condition: service_healthy
  redis:
    condition: service_healthy
  kafka:
    condition: service_healthy
```

Without this, `docker compose up` would start a service's container as soon as
Postgres/Redis/Kafka's *processes* started, not once they're actually accepting
connections — a Go binary that dials Postgres in its first line of `main()` would
crash-loop against a Postgres still running `initdb`. `notification-service`
doesn't depend on `postgres` (it has no database — see
[services/notification-service.md](../services/notification-service.md)).

Healthcheck commands used:

| Container | Healthcheck |
|---|---|
| `postgres` | `pg_isready -U saga` |
| `redis` | `redis-cli ping` |
| `kafka` | `kafka-broker-api-versions.sh --bootstrap-server localhost:9092` |

Even with all of this, a **fully cold** Kafka broker can take longer to finish
internal consumer-group rebalancing than `service_healthy` alone guarantees — see
[kafka.md](kafka.md#cold-start-behavior) for what that looks like and why it's not
a bug.

## Build context

Every service's `build.context` is `.` (the repo root), not the service's own
directory:

```yaml
booking-service:
  build:
    context: .
    dockerfile: services/booking-service/Dockerfile
```

This is required because of the `go.work` workspace layout — each service's
Dockerfile needs to `COPY` not just its own directory but `shared/` and, in
practice, all four `services/*` directories (since `go.work`'s `use` list
references all of them and `go build` needs every listed module's `go.mod` present
even if it doesn't import that module's code). See
[tech-stack.md](../tech-stack.md#language--module-layout) for why the workspace is
shaped this way, and each service's `Dockerfile` for the exact `COPY` list.

## Ports exposed to the host

| Service | Port | Purpose |
|---|---|---|
| `postgres` | `5432` | direct `psql`/`pgxpool` access for debugging |
| `redis` | `6379` | direct `redis-cli` access for debugging |
| `kafka` | `9092` | the `PLAINTEXT` listener services use; also usable from the host |
| `booking-service` | `8080` | the only HTTP API in the saga |

`driver-matching-service`, `payment-service`, and `notification-service` expose no
ports — they're pure Kafka consumers, reachable only through events.

## Volumes

Only Postgres has a named volume (`postgres-data`), so its data survives
`docker compose restart`/`stop`/`up` but is wiped by `docker compose down -v`.
Redis and Kafka have no volumes — both are treated as disposable in this demo (
Redis because it's only a 24-hour idempotency cache, Kafka because a demo doesn't
need to retain history across a full teardown).

## Running just the infra

```
docker compose up -d postgres redis kafka
```

Useful when developing a service locally with `go run ./cmd` against
containerized infra instead of rebuilding its Docker image on every change.

## `.dockerignore`

At the repo root, excludes `.git`, `**/*.feature`, `**/features/`, and
`**/*_test.go` from every build context — none of that is needed to build a
production binary, and excluding it keeps build contexts smaller and avoids
invalidating Docker's layer cache on test-only changes.
