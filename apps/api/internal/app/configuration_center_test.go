package app

import "testing"

func TestValidateOperationSettingsRequiresCoordinatesForDistanceCheck(t *testing.T) {
	input := storeOperationSettings{
		SettlementMode:               "PAY_BEFORE",
		OrderingMode:                 "MULTI_PERSON",
		DistanceCheckEnabled:         true,
		DistanceLimitM:               1000,
		OrderReminderIntervalMinutes: 5,
	}
	if err := validateOperationSettings(input); err == nil {
		t.Fatal("distance validation must fail closed without store coordinates")
	}
	latitude, longitude := 39.9042, 116.4074
	input.StoreLatitude, input.StoreLongitude = &latitude, &longitude
	if err := validateOperationSettings(input); err != nil {
		t.Fatalf("valid distance configuration rejected: %v", err)
	}
}

func TestValidateOperationSettingsAcceptsPayAfterMeal(t *testing.T) {
	input := storeOperationSettings{
		SettlementMode:               "PAY_AFTER",
		OrderingMode:                 "MULTI_PERSON",
		DistanceLimitM:               1000,
		OrderReminderIntervalMinutes: 5,
	}
	if err := validateOperationSettings(input); err != nil {
		t.Fatalf("pay-after-meal workflow should be available: %v", err)
	}
}

func TestSettlementPrintTriggerFollowsSettlementMode(t *testing.T) {
	if got := settlementPrintTrigger("PAY_AFTER"); got != "ORDER_CREATED" {
		t.Fatalf("pay-after mode must print when the order is submitted, got %s", got)
	}
	if got := settlementPrintTrigger("PAY_BEFORE"); got != "PAYMENT_SUCCESS" {
		t.Fatalf("pay-before mode must print after payment, got %s", got)
	}
}

func TestApplyOperationSettingsDefaultsRestoresDistanceWhenDisabled(t *testing.T) {
	input := storeOperationSettings{DistanceCheckEnabled: false}
	applyOperationSettingsDefaults(&input)
	if input.DistanceLimitM != 5000 {
		t.Fatalf("expected disabled distance check to retain a safe value, got %d", input.DistanceLimitM)
	}

	input.DistanceLimitM = -1
	applyOperationSettingsDefaults(&input)
	if input.DistanceLimitM != -1 {
		t.Fatal("explicit invalid values must still reach validation")
	}
}

func TestDistanceMeters(t *testing.T) {
	distance := distanceMeters(39.9042, 116.4074, 39.9051, 116.4074)
	if distance < 95 || distance > 105 {
		t.Fatalf("expected roughly 100m, got %.2fm", distance)
	}
}
