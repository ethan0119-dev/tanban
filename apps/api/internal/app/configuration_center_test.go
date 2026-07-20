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

func TestValidateOperationSettingsRejectsPayAfterMealUntilWorkflowExists(t *testing.T) {
	input := storeOperationSettings{
		SettlementMode:               "PAY_AFTER",
		OrderingMode:                 "MULTI_PERSON",
		DistanceLimitM:               1000,
		OrderReminderIntervalMinutes: 5,
	}
	if err := validateOperationSettings(input); err == nil {
		t.Fatal("pay-after-meal must stay unavailable until its order workflow exists")
	}
}

func TestDistanceMeters(t *testing.T) {
	distance := distanceMeters(39.9042, 116.4074, 39.9051, 116.4074)
	if distance < 95 || distance > 105 {
		t.Fatalf("expected roughly 100m, got %.2fm", distance)
	}
}
