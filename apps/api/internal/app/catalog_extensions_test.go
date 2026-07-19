package app

import "testing"

func TestValidateProductConfiguration(t *testing.T) {
	valid := productConfigurationInput{OptionGroups: []productOptionGroupInput{{
		Name: "甜度", Kind: "ATTRIBUTE", SelectionMode: "SINGLE", MinSelect: 1, MaxSelect: 1,
		Values: []productOptionValueInput{{Name: "标准糖"}, {Name: "无糖"}},
	}}}
	if err := validateProductConfiguration(&valid); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	invalid := valid
	invalid.OptionGroups[0].MaxSelect = 2
	if err := validateProductConfiguration(&invalid); err == nil {
		t.Fatal("single-select group with max_select=2 must be rejected")
	}

	insufficientActive := productConfigurationInput{OptionGroups: []productOptionGroupInput{{
		Name: "温度", SelectionMode: "SINGLE", MinSelect: 1, MaxSelect: 1,
		Values: []productOptionValueInput{{Name: "热", Status: "DISABLED"}},
	}}}
	if err := validateProductConfiguration(&insufficientActive); err == nil {
		t.Fatal("active group with fewer active values than min_select must be rejected")
	}

	tooManyDefaults := productConfigurationInput{OptionGroups: []productOptionGroupInput{{
		Name: "配料", SelectionMode: "MULTIPLE", MinSelect: 0, MaxSelect: 1,
		Values: []productOptionValueInput{{Name: "奶油", IsDefault: true}, {Name: "糖浆", IsDefault: true}},
	}}}
	if err := validateProductConfiguration(&tooManyDefaults); err == nil {
		t.Fatal("defaults beyond max_select must be rejected")
	}
}

func TestValidateModifierGroup(t *testing.T) {
	price := int64(100)
	valid := modifierGroupInput{Name: "加料", MinSelect: 0, MaxSelect: 2, Items: []modifierGroupItemInput{{ModifierItemID: 1, PriceOverride: &price}, {ModifierItemID: 2}}}
	if err := validateModifierGroup(&valid); err != nil {
		t.Fatalf("valid modifier group rejected: %v", err)
	}
	valid.Items = append(valid.Items, modifierGroupItemInput{ModifierItemID: 1})
	if err := validateModifierGroup(&valid); err == nil {
		t.Fatal("duplicate modifier item must be rejected")
	}

	defaults := modifierGroupInput{Name: "默认加料", MinSelect: 0, MaxSelect: 1, Items: []modifierGroupItemInput{{ModifierItemID: 1, IsDefault: true}, {ModifierItemID: 2, IsDefault: true}}}
	if err := validateModifierGroup(&defaults); err == nil {
		t.Fatal("default modifier items beyond max_select must be rejected")
	}
}

func TestNormalizeCatalogResourceInput(t *testing.T) {
	input := catalogResourceInput{ResourceType: "unit", Name: "杯"}
	if err := normalizeCatalogResourceInput(&input); err != nil {
		t.Fatal(err)
	}
	if input.ResourceType != "UNIT" || input.Status != "ACTIVE" || input.Config == nil {
		t.Fatalf("unexpected normalization: %#v", input)
	}
}
