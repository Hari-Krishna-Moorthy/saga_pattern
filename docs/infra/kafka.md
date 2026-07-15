# Kafka

One `apache/kafka:3.7.0` broker, KRaft mode (no ZooKeeper), auto-creates topics.
This is the event backbone the entire choreography saga runs over.

## KRaft, single node

```yaml
kafka:
  image: apache/kafka:3.7.0
  environment:
    KAFKA_NODE_ID: 1
    KAFKA_PROCESS_ROLES: broker,controller
    KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093,PLAINTEXT_HOST://:29092
    KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092,PLAINTEXT_HOST://localhost:29092
    KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
    KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
    KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
    KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
    KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
    CLUSTER_ID: ridebooking-saga-cluster
```

One process plays both `broker` and `controller` roles — this is what "KRaft"
(Kafka Raft metadata mode) replaces ZooKeeper with: the controller role manages
cluster metadata using a Raft quorum (`KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093`
— a quorum of one, since there's one broker). `KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR:
1` is required because a single-node cluster can't satisfy Kafka's normal default
replication factor of 3 for its internal `__consumer_offsets` topic.

Three listeners exist for three different audiences:

| Listener | Used by |
|---|---|
| `PLAINTEXT://:9092` | services, over the Docker Compose network (`kafka:9092`) |
| `CONTROLLER://:9093` | KRaft controller traffic only, internal |
| `PLAINTEXT_HOST://:29092` | the host machine, if you want to point a local tool at `localhost:29092` |

`KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"` means no topic-provisioning step exists
anywhere in this project — the first `Publish` or `NewReader` for a topic that
doesn't exist yet creates it with default settings (1 partition, replication
factor 1). Not what you'd want in production (you'd want explicit partition
counts sized to expected throughput, and you wouldn't want a typo in a topic name
to silently create a new topic instead of erroring) — fine for a demo.

## Topics

All eight event topics used by the saga, defined once as constants in
`shared/events/topics.go` — see
[shared-module.md](../shared-module.md#topics-topicsgo):

```
booking.requested   booking.cancelled
driver.matched      driver.match_failed      driver.released
payment.completed   payment.failed
ride.confirmed
```

## The consumer-group bug

Found during the first live end-to-end verification of the full saga, after all
four services' BDD suites were already green — a good example of why BDD tests
against fakes don't replace verifying the real wiring.

**Symptom:** `payment.completed` was published (visible on the topic via
`kafka-console-consumer`), but `booking-service` never confirmed the booking. No
errors were logged anywhere.

**Root cause:** `booking-service` subscribes to three different topics
(`payment.completed`, `driver.match_failed`, `payment.failed`), and the original
`internal/messaging/consumers.go` used **one Kafka consumer group ID**
(`"booking-service"`) for all three `Reader`s:

```go
// before the fix
reader := kafka.NewReader(brokers, topic, groupID) // groupID = "booking-service" for every topic
```

Kafka's consumer-group protocol expects every member of a group to subscribe to
the **same topic set** — that's the whole point of a group (coordinated partition
assignment across cooperating consumers of the same topic(s)). Here, three
`Reader`s joined the *same* group but each subscribed to a *different* topic. The
group coordinator ended up with three members whose subscriptions didn't agree,
and partition assignment silently broke: `kafka-consumer-groups.sh --describe
--group booking-service` showed all three members with **0 partitions assigned**,
and no message was ever delivered to any of them. No exception, no log line — just
permanently zero throughput.

**Fix:** give every topic its own group ID, scoped by service:

```go
// after the fix
reader := kafka.NewReader(brokers, topic, groupID+"."+topic)
// e.g. "booking-service.payment.completed", "booking-service.driver.match_failed", ...
```

Applied to every service subscribing to more than one topic: `booking-service`,
`driver-matching-service`, `notification-service`. (`payment-service` only
subscribes to one topic, `driver.matched`, so it was never affected — see
[services/payment-service.md](../services/payment-service.md#kafka-wiring-internalmessagingconsumersgo).)

Verified by re-running the full saga live afterward and watching
`kafka-consumer-groups.sh --describe` show a real partition assignment and nonzero
consumption for each group.

## Cold-start behavior

Separately from the bug above (which was a real logic error, now fixed): on a
**fully cold** broker — right after `docker compose down -v && docker compose up
-d --build` — the five consumer groups (one shared across the four services'
`payment-service`, plus per-topic groups for the other three) can take up to a
minute to finish their initial `JoinGroup`/`SyncGroup` rebalance and start
delivering messages, especially on resource-constrained hardware where multiple
JVMs (Kafka itself plus its startup class loading) are competing for CPU.

This is **not** the bug above — it reproduces even with the per-topic group ID fix
in place, it resolves itself without intervention (a booking created during that
window sits at `REQUESTED` and then completes once the consumers catch up), and it
only happens on a cold broker, not a warm one that's already been running.
Confirmed by restarting an individual service and observing its consumer group
stabilize (visible via `kafka-consumer-groups.sh --describe`) and start delivering
within roughly 15–30 seconds. See the root
[README.md](../../README.md#running-it) for the practical note this produced.

## Message key = booking ID

Every `Publish` call uses the event's `BookingID` as the Kafka message key (see
`shared/kafka.Writer.Publish`):

```go
kafkago.Message{
    Key:   []byte(evt.BookingID),
    Value: data,
}
```

This means every event for one booking is guaranteed to land on the same
partition and be delivered in publish order relative to each other — relevant
because, e.g., `booking-service` needs to see `booking.requested` (its own
publish) conceptually "before" it would ever see a reaction to it, and more
practically because it keeps all of one booking's saga history colocated for
easier debugging (`kafka-console-consumer --partition N` on one partition shows
one booking's full story if you know which partition its ID hashed to).

## At-least-once delivery, not exactly-once

`Reader.Run` (`shared/kafka/consumer.go`) commits the Kafka offset **only after**
the handler succeeds — if a handler returns an error, the offset isn't committed
and the same message is refetched on the next loop iteration. Combined with
consumer restarts (a message fetched but not yet committed when a service
crashes), this means every consumer must tolerate seeing the same event more than
once — which is exactly what `shared/idempotency` exists for. See
[shared-module.md#idempotency](../shared-module.md#sharedidempotency--dedup-guard)
and [redis.md](redis.md).
