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
