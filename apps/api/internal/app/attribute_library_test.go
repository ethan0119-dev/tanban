package app

import "testing"

func TestValidateAttributeGroup(t *testing.T) {
	input := attributeGroupInput{
		Name: " 温度 ", SelectionMode: "single", MinSelect: 1, MaxSelect: 1, Status: "active",
		Values: []attributeValueInput{
			{Name: "冰", Status: "active"},
			{Name: "热", IsDefault: true, Status: "active"},
		},
	}
	if err := validateAttributeGroup(&input); err != nil {
		t.Fatalf("valid attribute group rejected: %v", err)
	}
	if input.Name != "温度" || input.SelectionMode != "SINGLE" || input.Status != "ACTIVE" {
		t.Fatalf("attribute group was not normalized: %+v", input)
	}
}

func TestValidateAttributeGroupRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name  string
		input attributeGroupInput
	}{
		{
			name: "duplicate values",
			input: attributeGroupInput{Name: "甜度", SelectionMode: "SINGLE", MinSelect: 1, MaxSelect: 1, Values: []attributeValueInput{
				{Name: "无糖"}, {Name: "无糖"},
			}},
		},
		{
			name: "single selects more than one",
			input: attributeGroupInput{Name: "温度", SelectionMode: "SINGLE", MinSelect: 1, MaxSelect: 2, Values: []attributeValueInput{
				{Name: "冰"}, {Name: "热"},
			}},
		},
		{
			name: "too many defaults",
			input: attributeGroupInput{Name: "温度", SelectionMode: "SINGLE", MinSelect: 1, MaxSelect: 1, Values: []attributeValueInput{
				{Name: "冰", IsDefault: true}, {Name: "热", IsDefault: true},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAttributeGroup(&test.input); err == nil {
				t.Fatal("invalid attribute group was accepted")
			}
		})
	}
}
