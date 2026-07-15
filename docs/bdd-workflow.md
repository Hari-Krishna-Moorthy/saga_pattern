# BDD workflow

Every saga reaction in this project was built in the same order, in every service:

1. Write a `.feature` file (Gherkin: `Given`/`When`/`Then`) describing one scenario.
2. Write Go step definitions that call into that service's domain layer.
3. Run `go test` — it fails (either a compile error, because the method doesn't
   exist yet, or a red scenario).
4. Write just enough production code in `internal/domain/service.go` to make it
   pass.
5. Run `go test` again — green.
6. `git commit` — one commit per scenario (see the `git log` for the actual
   history: each commit message names the scenario it makes pass).

This repeated 11 times across the four services (see [features/](features/) for
every scenario). The pattern that makes step 4 possible without Docker, Kafka, or
Postgres running is described below.

## Why these tests don't need Docker

Every service's domain layer depends on **interfaces**, not concrete
infrastructure clients:

```go
type Repository interface {   // persistence port
    Save(ctx context.Context, b Booking) error
    FindByID(ctx context.Context, id string) (Booking, error)
    Update(ctx context.Context, b Booking) error
}

type Service struct {
    repo      Repository       // not *pgxpool.Pool
    publisher kafka.Publisher  // not *kafka.Writer directly — the Publisher interface
}
```

Production code (`internal/repository/postgres.go`) implements `Repository`
against real Postgres. BDD tests (`internal/domain/fakes_test.go`) implement the
*same interface* as an in-memory map guarded by a mutex:

```go
type fakeRepository struct {
    mu       sync.Mutex
    bookings map[string]domain.Booking
}

func (r *fakeRepository) Save(_ context.Context, b domain.Booking) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.bookings[b.ID] = b
    return nil
}
// ... FindByID, Update
```

Same shape for `kafka.Publisher` — a `fakePublisher` records every
`(topic, Envelope)` pair published in a slice instead of writing to Kafka, and
scenarios assert against that slice:

```go
func (p *fakePublisher) eventsOnTopic(topic string) []events.Envelope { ... }
```

Because `domain.Service` only knows about the interfaces, the exact same
`RequestBooking`, `HandlePaymentCompleted`, etc. methods run in both the test and
production paths — the test isn't testing a simplified stand-in for the logic,
it's testing the actual logic, with a fast, deterministic double standing in for
disk and network I/O. This is the classic **ports and adapters** (hexagonal
architecture) shape, applied at the scale of one microservice.

## Anatomy of a step-definition file

Every service has one `*_steps_test.go` file wiring Gherkin text to Go functions,
and one `TestFeatures` entry point that runs Godog as a regular Go test:

```go
func InitializeScenario(sc *godog.ScenarioContext) {
    t := &bookingTestCtx{}

    sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
        t.reset() // fresh fakes for every scenario — no state leaks between them
        return ctx, nil
    })

    sc.Step(`^I am a rider with id "([^"]*)"$`, t.iAmARiderWithID)
    sc.Step(`^I request a ride from "([^"]*)" to "([^"]*)"$`, t.iRequestARideFromTo)
    // ...
}

func TestFeatures(t *testing.T) {
    suite := godog.TestSuite{
        ScenarioInitializer: InitializeScenario,
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"../../features"},
            TestingT: t,
        },
    }
    if suite.Run() != 0 {
        t.Fatal("non-zero status returned, failed to run feature tests")
    }
}
```

`sc.Before` resetting all the fakes before every scenario is what makes scenario
order not matter — each scenario in a `.feature` file (there can be several, see
e.g. `cancel_booking.feature`'s two scenarios) gets a clean `fakeRepository` and
`fakePublisher`.

Running one service's suite:

```
cd services/booking-service && go test ./... -v
```

`-v` prints the full Gherkin scenario back with pass/fail per step, e.g.:

```
Scenario: A new ride request is stored and published
  Given I am a rider with id "rider-1"
  When I request a ride from "123 Main St" to "456 Oak Ave"
  Then the booking should be stored with status "REQUESTED"
  And a "booking.requested" event should be published for the booking
```

## Where the fakes stop and real infrastructure starts

`fakes_test.go` and the step-definition files live under `internal/domain/` and are
`_test.go` files — they never compile into the production binary. The real
adapters (`internal/repository/postgres.go`, `internal/messaging/consumers.go`,
`internal/httpapi/handler.go` where relevant) implement the same interfaces and are
wired together only in `cmd/main.go`, which is not exercised by `go test` at all.
That wiring was verified separately, live, against the full `docker-compose` stack,
for every service — see each service's doc under
[services/](services/) for the specific verification performed.
