package app

import "testing"

func TestResolveTenantDocumentDefinition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path   string
		column string
		name   string
		valid  bool
	}{
		{path: "business-license", column: "business_license_media_id", name: "营业执照", valid: true},
		{path: "FOOD-BUSINESS-LICENSE", column: "food_business_license_media_id", name: "食品经营许可证", valid: true},
		{path: "identity-card", valid: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			definition, ok := resolveTenantDocumentDefinition(test.path)
			if ok != test.valid {
				t.Fatalf("expected valid=%v, got %v", test.valid, ok)
			}
			if ok && (definition.Column != test.column || definition.Name != test.name) {
				t.Fatalf("unexpected definition: %#v", definition)
			}
		})
	}
}
