# Feature: Charging payment once a driver is matched

**File:** `services/payment-service/features/charge_payment.feature`
**Service:** `payment-service`
**Saga position:** step 3 — reacts to a driver match, either advances the saga
toward confirmation or triggers a cancellation.

```gherkin
Feature: Charging payment once a driver is matched
  As the payment service
  I want to charge the rider once a driver has been matched
  So that the saga can confirm the ride

  Scenario: The charge succeeds
    Given the rider's payment method will be accepted
    When driver "driver-1" is matched to booking "booking-1"
    Then a payment for booking "booking-1" should be recorded as "COMPLETED"
    And a "payment.completed" event should be published for booking "booking-1"

  Scenario: The charge is declined for insufficient funds
    Given the rider's payment method will be declined for insufficient funds
    When driver "driver-2" is matched to booking "booking-2"
    Then a payment for booking "booking-2" should be recorded as "FAILED"
    And a "payment.failed" event should be published for booking "booking-2"
```

## What it drives

`domain.Service.HandleDriverMatched(ctx, evt events.Envelope) error`
— see [services/payment-service.md](../services/payment-service.md#handledrivermatchedctx-evt-eventsenvelope-error).

## Step definitions (`internal/domain/payment_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `Given the rider's payment method will be accepted` | `theRidersPaymentMethodWillBeAccepted` | sets `fakeGateway.declineAll = false` |
| `Given the rider's payment method will be declined for insufficient funds` | `theRidersPaymentMethodWillBeDeclinedForInsufficientFunds` | sets `fakeGateway.declineAll = true` |
| `When driver "([^"]*)" is matched to booking "([^"]*)"` | `driverIsMatchedToBooking` | builds a `driver.matched` envelope with payload `{driver_id}`, calls `HandleDriverMatched` |
| `Then a payment for booking "([^"]*)" should be recorded as "([^"]*)"` | `paymentForBookingShouldBeRecordedAs` | looks up the fake repo's saved `Payment` for this booking, asserts `Status` |
| `And a "([^"]*)" event should be published for booking "([^"]*)"` | `anEventShouldBePublishedForBooking` | shared pattern, checks `payment.completed`/`payment.failed` |

## `fakeGateway` — the one test double with a controllable failure mode

Every other fake in this project (`fakeRepository`, `fakePublisher`) is a passive
recorder. `fakeGateway` is the one fake with actual test-controlled *behavior*,
because `payment-service`'s domain logic branches on a **third-party outcome**
(whether the charge succeeds), not just on data already in the repository:

```go
type fakeGateway struct {
    declineAll bool
}

func (g *fakeGateway) Charge(_ context.Context, _ string) error {
    if g.declineAll {
        return domain.ErrInsufficientFunds
    }
    return nil
}
```

The production equivalent, `internal/gateway.Simulated`, generalizes this same
idea (decline by booking ID rather than a blanket flag) so the failure branch can
also be exercised against the *live* Kafka/Postgres stack, not just in BDD tests —
see [services/payment-service.md#simulated-gateway](../services/payment-service.md#simulated-gateway-internalgatewaysimulatedgo).

## What happens after this, in the live saga

- `payment.completed` → `booking-service` confirms the booking, see
  [features/confirm-booking.md](confirm-booking.md).
- `payment.failed` → **two** services react independently: `booking-service`
  cancels the booking ([features/cancel-booking.md](cancel-booking.md)) and
  `driver-matching-service` releases the driver
  ([features/release-driver.md](release-driver.md)).
