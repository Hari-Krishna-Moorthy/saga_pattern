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
