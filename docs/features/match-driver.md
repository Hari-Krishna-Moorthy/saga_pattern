# Feature: Matching a driver to a requested booking

**File:** `services/driver-matching-service/features/match_driver.feature`
**Service:** `driver-matching-service`
**Saga position:** step 2 — reacts to the booking request, either advances the
saga toward payment or triggers a cancellation.

```gherkin
Feature: Matching a driver to a requested booking
  As the driver-matching service
  I want to assign an available driver to a requested booking
  So that the saga can proceed to taking payment

  Scenario: A driver is available
    Given a driver "driver-1" is available
    When booking "booking-1" is requested
    Then driver "driver-1" should be assigned to booking "booking-1"
    And a "driver.matched" event should be published for booking "booking-1"

  Scenario: No driver is available
    Given no drivers are available
    When booking "booking-2" is requested
    Then a "driver.match_failed" event should be published for booking "booking-2"
```

Both scenarios exercise the **same** production method — this feature is a good
example of one method with a branch, tested by two scenarios covering each branch,
rather than two methods.

## What it drives

`domain.Service.HandleBookingRequested(ctx, evt events.Envelope) error`
— see [services/driver-matching-service.md](../services/driver-matching-service.md#handlebookingrequestedctx-evt-eventsenvelope-error).

## Step definitions (`internal/domain/driver_matching_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `Given a driver "([^"]*)" is available` | `aDriverIsAvailable` | adds a `Driver{Status: AVAILABLE}` to the in-memory fake repository |
| `Given no drivers are available` | `noDriversAreAvailable` | no-op — the fake repository starts empty, so "no drivers" is just not calling the step above |
| `When booking "([^"]*)" is requested` | `bookingIsRequested` | builds a `booking.requested` envelope, calls `HandleBookingRequested` |
| `Then driver "([^"]*)" should be assigned to booking "([^"]*)"` | `driverShouldBeAssignedToBooking` | checks the fake repo: driver's `Status == MATCHED` and `AssignedBookingID` matches |
| `And a "([^"]*)" event should be published for booking "([^"]*)"` | `anEventShouldBePublishedForBooking` | scans the fake publisher for this topic + booking ID |

Note the "no drivers" `Given` step doing nothing is intentional and documented
inline in the step definition — it reads clearly in the Gherkin even though there's
no corresponding state change, because the *absence* of a driver is the default
state of a fresh fake repository.

## The two commits behind this file

The first commit added the `Given a driver is available` scenario along with
`HandleBookingRequested`'s full implementation (both branches — the method can't
usefully be split into "just the success path"). The second commit added the "no
drivers" scenario against the **same** already-written method — i.e. that commit
added zero new production code, only a new test proving the failure branch already
present in the method actually works. See `git log --oneline`.

## What happens after this, in the live saga

- `driver.matched` → `payment-service` reacts, see
  [features/charge-payment.md](charge-payment.md).
- `driver.match_failed` → `booking-service` reacts by cancelling the booking, see
  [features/cancel-booking.md](cancel-booking.md).
