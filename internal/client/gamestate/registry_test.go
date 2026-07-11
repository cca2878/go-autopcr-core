package gamestate

import (
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
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
