package app

import "testing"

func TestNormalizeStoreProfile(t *testing.T) {
	latitude, longitude := 39.143528, 117.17147
	item, err := normalizeStoreProfile(merchantStoreProfile{
		VisibleInMiniapp:     true,
		ContactName:          " 码农咖啡 ",
		Region:               " 天津 / 天津市 / 红桥区 ",
		MainProducts:         " 美式、拿铁 ",
		AverageSpendCents:    1800,
		ServiceChannels:      []string{"DINE_IN", "TAKEOUT"},
		EnvironmentImageURLs: []string{"https://example.com/store.jpg"},
		FoodSafetyImageURLs:  []string{"https://example.com/safety.jpg"},
		StoreLatitude:        &latitude,
		StoreLongitude:       &longitude,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ContactName != "码农咖啡" || item.Region != "天津 / 天津市 / 红桥区" || len(item.ServiceChannels) != 2 {
		t.Fatalf("unexpected normalized profile: %+v", item)
	}
}

func TestNormalizeStoreProfileRejectsIncompleteLocationAndDuplicateImages(t *testing.T) {
	latitude := 39.143528
	if _, err := normalizeStoreProfile(merchantStoreProfile{ServiceChannels: []string{"TAKEOUT"}, StoreLatitude: &latitude}); err == nil {
		t.Fatal("incomplete coordinates must be rejected")
	}
	if _, err := normalizeStoreProfile(merchantStoreProfile{
		ServiceChannels:      []string{"TAKEOUT"},
		EnvironmentImageURLs: []string{"https://example.com/a.jpg", "https://example.com/a.jpg"},
	}); err == nil {
		t.Fatal("duplicate images must be rejected")
	}
}
