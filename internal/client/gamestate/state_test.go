package gamestate

import "testing"

// mana/jewel 不进 Inventory 表而落在 Gold/Jewel 专用字段，GetInventory 必须按 ref 特判——
// 漏了这条，彩装究极炼成读到的 mana 恒为 0，模块永远进不到第一发。
func TestGetInventorySpecialCurrencies(t *testing.T) {
	s := New()
	s.Gold = Currency{Free: 1_500_000}
	s.Jewel = Currency{Paid: 1000, Free: 300}
	s.Inventory = map[InventoryKey]int{{Type: 2, ID: 26202}: 42}

	if got := s.GetInventory(12, 94000); got != 1_500_000 { // zmana
		t.Fatalf("zmana=%d want 1500000", got)
	}
	if got := s.GetInventory(12, 94002); got != 1_500_000 { // mana
		t.Fatalf("mana=%d want 1500000", got)
	}
	if got := s.GetInventory(8, 91002); got != 1300 { // jewel = paid + free（两段互斥）
		t.Fatalf("jewel=%d want 1300", got)
	}
	if got := s.GetInventory(2, 26202); got != 42 { // 普通物品仍走库存表
		t.Fatalf("普通物品=%d want 42", got)
	}
}
