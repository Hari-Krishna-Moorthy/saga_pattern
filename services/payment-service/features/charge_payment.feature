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
