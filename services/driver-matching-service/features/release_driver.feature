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
