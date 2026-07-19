package app

import (
	"strings"
	"testing"
)

func TestRenderTicketIncludesProductConfiguration(t *testing.T) {
	order := orderDTO{OrderNo: "TB1", TotalCents: 1800, Items: []orderItemDTO{{
		ProductName: "拿铁", SKUName: "大杯", Quantity: 1, ItemRemark: "奶泡少一点",
		Configuration: map[string]any{
			"options":   []any{map[string]any{"groupName": "温度", "valueName": "冰"}},
			"modifiers": []any{map[string]any{"name": "浓缩", "quantity": float64(2)}},
		},
	}}}
	result := renderTicket("{{items}}", order, "", false)
	for _, expected := range []string{"拿铁 大杯 x1", "温度：冰", "加料：浓缩x2", "单品备注：奶泡少一点"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("ticket missing %q: %s", expected, result)
		}
	}
}
