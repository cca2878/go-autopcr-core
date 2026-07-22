package gamestate

import (
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
)

func TestApplyFoldsLoadIndex(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	r.Apply(s, &account.LoadIndexResponse{
		UserInfo:  &account.UserInfo{ViewerID: 123, UserName: "骑士君", TeamLevel: 200, UserStamina: 50},
		UserJewel: &account.UserJewel{Jewel: 1000, FreeJewel: 300},
		UserGold:  &account.UserGold{GoldIDPay: 100, GoldIDFree: 900},
	})
	if s.UserName != "骑士君" || s.ViewerID != 123 || s.TeamLevel != 200 || s.Stamina != 50 {
		t.Fatalf("user_info 未正确折叠: %+v", s)
	}
	if s.Jewel.Total != 1000 || s.Jewel.Free != 300 {
		t.Fatalf("jewel 折叠错误: %+v", s.Jewel)
	}
	if s.Gold != 1000 {
		t.Fatalf("gold=%d want 1000", s.Gold)
	}
}

func TestApplyFoldsMaintenance(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	r.Apply(s, &sdk.SourceIniGetMaintenanceStatusResponse{ResVer: "10002200", ManifestVer: "abc"})
	if s.ResVer != "10002200" || s.ManifestVer != "abc" {
		t.Fatalf("maintenance 折叠错误: %+v", s)
	}
}

func TestApplyIgnoresUnregistered(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	// 未登记类型：不应 panic，也不应改动状态。
	r.Apply(s, &sdk.CheckGameStartResponse{NowTutorial: true})
	if s.UserName != "" || s.ViewerID != 0 {
		t.Fatalf("未登记类型不应改动状态: %+v", s)
	}
}

// mana/jewel 不进 Inventory 表而落在 Gold/Jewel 专用字段，GetInventory 必须按 ref 特判——
// 漏了这条，彩装究极炼成读到的 mana 恒为 0，模块永远进不到第一发。
func TestGetInventorySpecialCurrencies(t *testing.T) {
	s := New()
	s.Gold = 1_500_000
	s.Jewel = Jewel{Total: 1000, Free: 300}
	s.Inventory = map[InventoryKey]int{{Type: 2, ID: 26202}: 42}

	if got := s.GetInventory(12, 94000); got != 1_500_000 { // zmana
		t.Fatalf("zmana=%d want 1500000", got)
	}
	if got := s.GetInventory(12, 94002); got != 1_500_000 { // mana
		t.Fatalf("mana=%d want 1500000", got)
	}
	if got := s.GetInventory(8, 91002); got != 1300 { // jewel = total + free
		t.Fatalf("jewel=%d want 1300", got)
	}
	if got := s.GetInventory(2, 26202); got != 42 { // 普通物品仍走库存表
		t.Fatalf("普通物品=%d want 42", got)
	}
}

// exec 回传的 user_gold 必须折回：模块每发都带 current_gold 快照，漏折则第二发起就是过期值。
func TestApplyFoldsAlcesExecGold(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	s.Gold = 1_000_000
	s.Inventory = map[InventoryKey]int{}
	r.Apply(s, &alces.ExecResponse{
		CurrentAlcesPoint: &alces.InventoryInfo{Type: 2, ID: 26202, Stock: 7},
		UserGold:          &protocol.UserGold{GoldIDPay: 100_000, GoldIDFree: 800_000},
	})
	if s.Gold != 900_000 {
		t.Fatalf("gold=%d want 900000", s.Gold)
	}
	if s.Inventory[InventoryKey{Type: 2, ID: 26202}] != 7 {
		t.Fatalf("炼成 PT 未折叠: %+v", s.Inventory)
	}
}

// 点赞/赛马抽取都必须回写守卫依据，否则同一会话内重跑会绕过守卫、撞上业务错误码。
func TestApplyFoldsGuardsAfterWrites(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	s.CharaFortune = &CharaFortune{FortuneID: 1, UnitID: 100101, Rank: 3}

	r.Apply(s, &clan.ClanLikeResponse{})
	if s.ClanLikeCount != 1 {
		t.Fatalf("ClanLikeCount=%d want 1", s.ClanLikeCount)
	}
	r.Apply(s, &race.CharaFortuneDrawResponse{})
	if s.CharaFortune != nil {
		t.Fatalf("赛马抽取后 CharaFortune 应清空: %+v", s.CharaFortune)
	}
}

func TestApplyNilPartialFields(t *testing.T) {
	r := DefaultRegistry()
	s := New()
	// user_jewel / user_gold 缺失时不应 panic。
	r.Apply(s, &account.LoadIndexResponse{
		UserInfo: &account.UserInfo{UserName: "无钻玩家"},
	})
	if s.UserName != "无钻玩家" || s.Jewel.Total != 0 || s.Gold != 0 {
		t.Fatalf("部分字段缺失处理错误: %+v", s)
	}
}
