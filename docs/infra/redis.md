# Redis

One `redis:7-alpine` container, shared by all four services, used for **exactly
one purpose**: making each service's Kafka consumers idempotent.

```yaml
redis:
  image: redis:7-alpine
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
```

No volume — Redis's data is disposable. If it's wiped (container recreated), the
worst case is a handful of already-processed events being reprocessed once more
before their keys are rewritten, which every consumer is already built to tolerate
safely (see below) — it's a cache, not a source of truth.

## What it's *not* used for

Deliberately not a cache for query results, not a session store, not a distributed
lock, not a rate limiter. One shared instance, one job. If this project grew a
second Redis use case, it would be worth asking whether that use case actually
needs the *same* Redis instance/config as the idempotency guard, or whether they'd
be better isolated.

## The idempotency mechanism

Full implementation in [shared-module.md](../shared-module.md#sharedidempotency--dedup-guard);
summarized here from the Redis side.

Every consumer, in every service, does this before running its actual saga logic:

```go
seen, err := idem.AlreadyProcessed(ctx, evt.EventID)
if seen {
    return nil // skip — already handled this exact event
}
// ... call the real domain method
```

Which runs one Redis command:

```
SETNX <prefix>:<event-id> 1
EXPIRE <prefix>:<event-id> 86400   (24h, set via SET ... EX in the go-redis call)
```

`SETNX` ("set if not exists") is atomic — there's no read-then-write race between
two goroutines (or two service instances) seeing the same event concurrently and
both deciding "not seen yet." Whichever call's `SETNX` actually creates the key
wins the right to process that event; the other sees the key already exists and
skips.

## Key scheme: one prefix per service

Each service passes its own `IDEMPOTENCY_PREFIX` (env var, defaulting to the
service name — see each service's config table under
[docs/services/](../services/)) when constructing its `RedisChecker`:

```go
idem := idempotency.NewRedisChecker(redisClient, idemPrefix)
```

So the same event ID, if it happened to be relevant to two different services (it
never is in this project — every event has exactly one topic and each topic has
consumers in only one or two services, each keeping separate track), would never
collide, because the actual Redis key is `<prefix>:<event-id>`, e.g.:

```
booking-service:3570ff54-e90b-48f1-8c16-91937a0b8454
payment-service:c011f4f7-f94e-4bfc-b868-5dd37f143b5b
```

More importantly: within *one* service that subscribes to multiple topics (e.g.
`booking-service` subscribing to `payment.completed`, `driver.match_failed`, and
`payment.failed`), all three consumers share the same prefix and the same
`RedisChecker` instance — which is correct, because event IDs are globally unique
UUIDs (see `events.NewEnvelope`), so there's no need to further namespace by topic.

## TTL: why 24 hours

The `SETNX` key expires after 24 hours (`RedisChecker.ttl`, hardcoded in
`shared/idempotency/redis.go`). A booking's entire saga — request, match, charge,
confirm/cancel, notify — completes in seconds in this project. 24 hours is a large
safety margin against Redis memory growth being unbounded (every event ever
published would otherwise leave a key forever), not a tight deadline being cut
close. If Kafka ever redelivered a message more than 24 hours after it was first
processed (a very unusual scenario — it would mean a consumer group's offset
hadn't advanced in over a day), that redelivery would be treated as new and
reprocessed; every domain method in this project is naturally idempotent at the
business-logic level too where it matters (e.g. re-confirming an already-CONFIRMED
booking just sets the same status again), so this isn't a correctness cliff, just
a design choice that trades a tiny window of possible double-processing for
bounded memory use.
