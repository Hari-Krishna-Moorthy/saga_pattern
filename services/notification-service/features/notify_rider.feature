Feature: Notifying the rider of saga outcomes
  As a rider
  I want to be notified when my ride is confirmed or cancelled
  So that I know what's happening without checking the app

  Scenario: The ride is confirmed
    When the ride is confirmed for booking "booking-1"
    Then the rider should be notified for booking "booking-1" with a message containing "confirmed"
