# Feature: Requesting a ride

**File:** `services/booking-service/features/request_booking.feature`
**Service:** `booking-service`
**Saga position:** the trigger — this is how every saga run begins.

```gherkin
Feature: Requesting a ride
  As a rider
  I want to request a ride
  So that the saga can find me a driver and charge me for the trip

  Scenario: A new ride request is stored and published
    Given I am a rider with id "rider-1"
    When I request a ride from "123 Main St" to "456 Oak Ave"
    Then the booking should be stored with status "REQUESTED"
    And a "booking.requested" event should be published for the booking
```

## What it drives

`domain.Service.RequestBooking(ctx, riderID, pickup, dropoff string) (Booking, error)`
— see [services/booking-service.md](../services/booking-service.md#requestbookingctx-riderid-pickup-dropoff-string-booking-error).

## Step definitions (`internal/domain/booking_steps_test.go`)

| Step | Go function | What it does |
|---|---|---|
| `Given I am a rider with id "([^"]*)"` | `iAmARiderWithID` | stores the rider ID on the test context — no production call yet |
| `When I request a ride from "([^"]*)" to "([^"]*)"` | `iRequestARideFromTo` | calls `service.RequestBooking(ctx, riderID, pickup, dropoff)`, stores the returned `Booking` and any error |
| `Then the booking should be stored with status "([^"]*)"` | `theBookingShouldBeStoredWithStatus` | `repo.FindByID` on the in-memory fake, asserts `Status` matches |
| `And a "([^"]*)" event should be published for the booking` | `anEventShouldBePublishedForTheBooking` | scans the fake publisher's recorded events for this topic + this booking's ID |

## Why this scenario alone doesn't need a `Given` for the booking

Unlike every other feature in this project, this one doesn't start from an
existing booking — creating the booking *is* the action under test. Every other
booking-service scenario (`confirm-booking`, `cancel-booking`) reuses
`I request a ride from "..." to "..."` as a `Given` step to set up its
precondition, which is why that step definition is shared across all three
`.feature` files in `booking-service` — they all compile against one
`InitializeScenario` function.

## What happens after this, in the live saga

`booking.requested` is what `driver-matching-service` subscribes to — see
[features/match-driver.md](match-driver.md). This scenario only tests up to the
publish; the rest of the saga is each downstream service's own BDD suite.
