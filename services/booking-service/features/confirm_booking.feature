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
