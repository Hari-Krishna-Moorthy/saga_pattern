# Feature: Confirming a booking after payment

**File:** `services/booking-service/features/confirm_booking.feature`
**Service:** `booking-service`
**Saga position:** the happy-path terminal state.

```gherkin
Feature: Confirming a booking after payment
  As the booking service
  I want to confirm a booking once payment succeeds
  So that the rider and driver both know the ride is on

  Scenario: Payment completes for a requested booking
    Given I am a rider with id "rider-1"
    And I request a ride from "123 Main St" to "456 Oak Ave"
    When payment completes for the booking
    Then the booking should be stored with status "CONFIRMED"
    And a "ride.confirmed" event should be published for the booking
```

## What it drives

`domain.Service.HandlePaymentCompleted(ctx, evt events.Envelope) error`
— see [services/booking-service.md](../services/booking-service.md#handlepaymentcompletedctx-evt-eventsenvelope-error).

## Step definitions (`internal/domain/booking_steps_test.go`)

The `Given`/`And` steps (`iAmARiderWithID`, `iRequestARideFromTo`) are the same two
steps used in [request-booking.md](request-booking.md) — this scenario's
precondition *is* that other feature's action, run first to get a real booking ID
to react against.

| Step | Go function | What it does |
|---|---|---|
| `When payment completes for the booking` | `paymentCompletesForTheBooking` | builds a `payment.completed` `events.Envelope` for `t.booking.ID` and calls `service.HandlePaymentCompleted(ctx, evt)` directly — no real Kafka involved |
| `Then the booking should be stored with status "([^"]*)"` | `theBookingShouldBeStoredWithStatus` | shared with `request-booking.md` — same assertion function, different expected status |
| `And a "([^"]*)" event should be published for the booking` | `anEventShouldBePublishedForTheBooking` | shared assertion function, checks for `ride.confirmed` this time |

## Notable: the event payload isn't inspected

`payment.completed` and `ride.confirmed` are both published with an empty
`struct{}{}` payload in production — everything either side needs is the
`BookingID` already on the envelope. This scenario reflects that: the step
definition builds the incoming event with `struct{}{}` too, matching what the real
`payment-service` actually publishes (see
[features/charge-payment.md](charge-payment.md)).

## What happens after this, in the live saga

`ride.confirmed` is what `notification-service` subscribes to, to send
`"Your ride is confirmed!"` — see [features/notify-rider.md](notify-rider.md).
