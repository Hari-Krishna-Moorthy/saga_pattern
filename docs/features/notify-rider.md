# Feature: Notifying the rider of saga outcomes

**File:** `services/notification-service/features/notify_rider.feature`
**Service:** `notification-service`
**Saga position:** the very end of both possible saga outcomes — nothing consumes
what this service produces.

```gherkin
Feature: Notifying the rider of saga outcomes
  As a rider
  I want to be notified when my ride is confirmed or cancelled
  So that I know what's happening without checking the app

  Scenario: The ride is confirmed
    When the ride is confirmed for booking "booking-1"
    Then the rider should be notified for booking "booking-1" with a message containing "confirmed"

  Scenario: The booking is cancelled
    When booking "booking-2" is cancelled with reason "no driver available"
    Then the rider should be notified for booking "booking-2" with a message containing "no driver available"
```

Note there's no `Given` step in either scenario — `notification-service` is
stateless, so there's no precondition to set up beyond constructing a fresh
`Service` (done in `sc.Before`, same as every other service).

## What it drives

Two separate methods, one per scenario:

| Scenario | Method |
|---|---|
| The ride is confirmed | `HandleRideConfirmed(ctx, evt events.Envelope) error` |
| The booking is cancelled | `HandleBookingCancelled(ctx, evt events.Envelope) error` |

See [services/notification-service.md](../services/notification-service.md) for
full method docs.

## Step definitions (`internal/domain/notification_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `When the ride is confirmed for booking "([^"]*)"` | `theRideIsConfirmedForBooking` | builds a `ride.confirmed` envelope, calls `HandleRideConfirmed` |
| `When booking "([^"]*)" is cancelled with reason "([^"]*)"` | `bookingIsCancelledWithReason` | builds a `booking.cancelled` envelope **with a `{reason}` payload**, calls `HandleBookingCancelled` |
| `Then the rider should be notified for booking "([^"]*)" with a message containing "([^"]*)"` | `theRiderShouldBeNotifiedForBookingWithMessageContaining` | scans the fake notifier's recorded `(bookingID, message)` pairs, asserts a substring match |

The assertion step uses `strings.Contains` rather than an exact string match
deliberately — it's checking that the reason text made it into the message (e.g.
`"no driver available"` appears somewhere in `"Your ride was cancelled: no driver
available"`), not pinning the exact wording of the surrounding sentence.

## Why the second scenario's `Given` step is a payload, not repo state

Compare this to every other service's scenarios: `HandleBookingCancelled` doesn't
look anything up — it only reads `evt.Payload` (for the `reason`) and
`evt.BookingID`, then calls `Notifier.Send`. So this step definition builds the
envelope's payload directly (`struct{ Reason string }{Reason: reason}`) rather than
composing prior steps to build up state, because there *is* no state to build up —
this is the simplest domain method in the whole project.

## Where the saga ends

Nothing subscribes to anything `notification-service` produces (it doesn't publish
to Kafka at all — see [services/notification-service.md](../services/notification-service.md#log-notifier-internalnotifierlogo)).
This is the terminal point of both saga paths documented in
[architecture.md](../architecture.md#event-flow-happy-path).
