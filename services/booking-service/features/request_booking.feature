Feature: Requesting a ride
  As a rider
  I want to request a ride
  So that the saga can find me a driver and charge me for the trip

  Scenario: A new ride request is stored and published
    Given I am a rider with id "rider-1"
    When I request a ride from "123 Main St" to "456 Oak Ave"
    Then the booking should be stored with status "REQUESTED"
    And a "booking.requested" event should be published for the booking
