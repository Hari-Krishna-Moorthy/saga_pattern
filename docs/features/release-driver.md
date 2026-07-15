# Feature: Releasing a driver when payment fails

**File:** `services/driver-matching-service/features/release_driver.feature`
**Service:** `driver-matching-service`
**Saga position:** compensation — `driver-matching-service`'s independent reaction
to a payment decline.

```gherkin
Feature: Releasing a driver when payment fails
  As the driver-matching service
  I want to release a driver back to the pool if payment fails
  So that the driver becomes available for other bookings

  Scenario: Payment fails after a driver was matched
    Given a driver "driver-1" is available
    And booking "booking-1" is requested
    When payment fails for booking "booking-1"
    Then driver "driver-1" should be available again
    And a "driver.released" event should be published for booking "booking-1"
```

## What it drives

`domain.Service.HandlePaymentFailed(ctx, evt events.Envelope) error`
— see [services/driver-matching-service.md](../services/driver-matching-service.md#handlepaymentfailedctx-evt-eventsenvelope-error).

This method is distinct from `booking-service`'s method of the same name
(`HandlePaymentFailed`) — same Kafka topic (`payment.failed`), two completely
separate implementations in two separate services, each doing its own
compensation. See [features/cancel-booking.md](cancel-booking.md) for the
`booking-service` side of this same event.

## Step definitions (`internal/domain/driver_matching_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `Given a driver "([^"]*)" is available` | `aDriverIsAvailable` | shared with [match-driver.md](match-driver.md) |
| `And booking "([^"]*)" is requested` | `bookingIsRequested` | shared — this is the setup: run the driver-matching happy path first so there's a real `MATCHED` driver to release |
| `When payment fails for booking "([^"]*)"` | `paymentFailsForBooking` | builds a `payment.failed` envelope, calls `HandlePaymentFailed` |
| `Then driver "([^"]*)" should be available again` | `driverShouldBeAvailableAgain` | checks the fake repo: `Status == AVAILABLE` and `AssignedBookingID == ""` |
| `And a "([^"]*)" event should be published for booking "([^"]*)"` | `anEventShouldBePublishedForBooking` | shared, checks `driver.released` |

The `Given .../And ...` pair here composes two prior steps from
[match-driver.md](match-driver.md) to build up realistic state — the scenario
doesn't fabricate a `MATCHED` driver directly, it drives the actual
`HandleBookingRequested` method first, so the "driver was matched" precondition is
proven by the same production code this project already tests elsewhere, not
hand-set fixture data that could drift from reality.

## What happens after this, in the live saga

`driver.released` has no subscriber in this project — it's published for
observability/extensibility (a future driver-facing notification, or an
analytics/reporting consumer, could subscribe without any existing service
changing). Verified live: forcing a payment decline via `payment-service`'s
`DECLINE_BOOKING_IDS` and confirming the previously-`MATCHED` driver flipped back
to `AVAILABLE` in Postgres.
