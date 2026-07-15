# Postgres

One `postgres:16-alpine` container, three databases, database-per-service.

## Database-per-service, one container

```yaml
postgres:
  image: postgres:16-alpine
  environment:
    POSTGRES_USER: saga
    POSTGRES_PASSWORD: saga
    POSTGRES_DB: postgres
  volumes:
    - postgres-data:/var/lib/postgresql/data
    - ./deploy/postgres/init-databases.sh:/docker-entrypoint-initdb.d/init-databases.sh
```

`POSTGRES_DB: postgres` creates the default administrative database. The three
service databases — `booking_service`, `driver_matching_service`,
`payment_service` — are created by `deploy/postgres/init-databases.sh`, which
Postgres's official image runs automatically on **first boot only** (any script in
`/docker-entrypoint-initdb.d/` runs once, when the data directory is empty; it will
not re-run on a container restart against an existing volume).

```bash
#!/usr/bin/env bash
set -euo pipefail

for db in booking_service driver_matching_service payment_service; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'CREATE DATABASE $db'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
EOSQL
done
```

`notification-service` has no database — it's the one stateless service.

## A bug found during first live verification

The very first version of this script called `psql --username "$POSTGRES_USER"`
**without** `--dbname`. `psql` without an explicit database defaults to a database
named after the connecting role — so it tried to connect to a database literally
named `saga` (the role name), which doesn't exist, and the whole init script failed
before creating any of the three real databases:

```
FATAL:  database "saga" does not exist
```

Fixed by adding `--dbname "$POSTGRES_DB"` (which resolves to `postgres`, the
always-present administrative database) so the script has somewhere to actually
connect and run `CREATE DATABASE` from. Caught by running `docker compose up -d
postgres` and checking `\l` before any service was even built — a reminder that
"the container is `Up (healthy)`" and "the container did the thing it was supposed
to do" are different claims; the healthcheck (`pg_isready`) only proves Postgres
*itself* started, not that the init script succeeded.

## Client: pgx, not `database/sql`

Every service's `internal/repository/postgres.go` uses
[`jackc/pgx/v5/pgxpool`](https://github.com/jackc/pgx) directly — `pgxpool.Pool`,
`pool.Exec`, `pool.QueryRow` — not the standard library's `database/sql` package.
See [tech-stack.md](../tech-stack.md#postgres--durable-state-one-database-per-service)
for why.

## Migrations: embedded, not a separate tool

No `golang-migrate`/`goose`/etc. Each service has a `migrations/` directory with
plain `.sql` files and a `migrations.go` that embeds them:

```go
//go:embed *.sql
var Files embed.FS
```

`cmd/main.go` reads the embedded directory, sorts filenames, and `pool.Exec`s each
file's contents in order, on every startup:

```go
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    entries, _ := migrations.Files.ReadDir(".")
    names := make([]string, 0, len(entries))
    for _, e := range entries {
        names = append(names, e.Name())
    }
    sort.Strings(names)
    for _, name := range names {
        sqlBytes, _ := migrations.Files.ReadFile(name)
        if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
            return err
        }
    }
    return nil
}
```

Every migration file uses `CREATE TABLE IF NOT EXISTS` (and, for
`driver-matching-service`'s seed data, `INSERT ... ON CONFLICT DO NOTHING`), so
re-running them on every startup is safe and idempotent — there's no "has this
migration already run" bookkeeping table, because there's no need for one at this
scale. This trades away rollback support and multi-step schema evolution for a
service binary that's fully self-contained: deploying a new version of a service
is just "run the new binary," nothing else.

## Concurrency note: `AssignDriver`'s conditional update

`driver-matching-service`'s `PostgresRepository.AssignDriver` guards against two
concurrent bookings racing for the same driver with a conditional `UPDATE`, not a
`SELECT ... FOR UPDATE` transaction:

```sql
UPDATE drivers SET status = 'MATCHED', assigned_booking_id = $3
WHERE id = $1 AND status = 'AVAILABLE'
```

If this affects 0 rows, the driver was already taken between `FindAvailableDriver`
and `AssignDriver` (a real, if narrow, race window — those are two separate
queries, not one transaction) and an error is returned. This is a "good enough for
a demo" guard: it prevents double-booking a driver, but under real concurrent load
you'd want `FindAvailableDriver` itself to use `SELECT ... FOR UPDATE SKIP LOCKED`
inside the same transaction as the assignment, so a failed race doesn't surface as
an error to the caller — it would just try the next available driver instead.
