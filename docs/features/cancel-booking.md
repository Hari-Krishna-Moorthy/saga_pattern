# Feature: Cancelling a booking when the saga can't complete

**File:** `services/booking-service/features/cancel_booking.feature`
**Service:** `booking-service`
**Saga position:** both of this saga's compensated terminal states, from
`booking-service`'s side.

```gherkin
Feature: Cancelling a booking when the saga can't complete
  As the booking service
  I want to cancel a booking if any downstream saga step fails
  So that the rider is never left in limbo

  Scenario: No driver is available for a requested booking
    Given I am a rider with id "rider-1"
    And I request a ride from "123 Main St" to "456 Oak Ave"
    When no driver is available for the booking
    Then the booking should be stored with status "CANCELLED"
    And a "booking.cancelled" event should be published for the booking

  Scenario: Payment fails after a driver was matched
    Given I am a rider with id "rider-1"
    And I request a ride from "123 Main St" to "456 Oak Ave"
    When payment fails for the booking
    Then the booking should be stored with status "CANCELLED"
    And a "booking.cancelled" event should be published for the booking
```

Two scenarios, two different upstream failures, both ending the same way for
`booking-service` — which is exactly why both were built to share one private
helper in production code.

## What it drives

Both scenarios ultimately exercise `cancelBooking(ctx, bookingID, reason string)`
(private), via two different public entry points:

| Scenario | Public method | `cancelBooking` reason |
|---|---|---|
| No driver is available | `HandleDriverMatchFailed(ctx, evt)` | `"no driver available"` |
| Payment fails after a driver was matched | `HandlePaymentFailed(ctx, evt)` | `"payment failed"` |

See [services/booking-service.md](../services/booking-service.md) for the full
method docs.

## Step definitions (`internal/domain/booking_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `When no driver is available for the booking` | `noDriverIsAvailableForTheBooking` | builds a `driver.match_failed` envelope, calls `HandleDriverMatchFailed` |
| `When payment fails for the booking` | `paymentFailsForTheBooking` | builds a `payment.failed` envelope, calls `HandlePaymentFailed` |
| `Then the booking should be stored with status "([^"]*)"` | `theBookingShouldBeStoredWithStatus` | shared, asserts `CANCELLED` |
| `And a "([^"]*)" event should be published for the booking` | `anEventShouldBePublishedForTheBooking` | shared, asserts `booking.cancelled` was published |

Neither scenario asserts on `CancelReason` directly (there's no `Then the cancel
reason should be "..."` step) — the reason is exercised implicitly through
`notification-service`'s own scenarios (see
[features/notify-rider.md](notify-rider.md)), which *do* assert on the reason
text, using the `booking.cancelled` payload `booking-service` actually publishes.

## The two commits behind this file

Built as two separate commits despite being one `.feature` file: the "no driver"
scenario landed first (exercising `HandleDriverMatchFailed`, which already existed
as the fallback branch of `HandleBookingRequested` — no *new* production code was
needed on the driver-matching side, but `booking-service` needed
`HandleDriverMatchFailed` written for the first time). The "payment fails"
scenario landed second, adding `HandlePaymentFailed` and the shared
`cancelBooking` helper. See `git log --oneline` for the commit messages.

## What happens after this, in the live saga

`booking.cancelled` is what `notification-service` subscribes to (see
[features/notify-rider.md](notify-rider.md)). For the payment-failure path
specifically, `driver-matching-service` *also* reacts to the underlying
`payment.failed` event independently — see
[features/release-driver.md](release-driver.md) — to release the driver it had
reserved. Neither service calls the other.
