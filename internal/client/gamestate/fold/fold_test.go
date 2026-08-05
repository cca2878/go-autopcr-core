package fold_test

import (
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate/fold"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/mirage"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/room"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/seasonpass"
)

// 通用层不认响应类型，只认 RewardCarrier——响应实现了接口就该被折叠，无需在注册表登记。
func TestFoldRewardsNeedsNoRegistration(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	// room 的收取响应'没有'域专属折叠器登记，只靠实现接口被通用层折叠。
	r.Apply(s, nil, &room.RoomReceiveItemAllResponse{
		RewardList: []protocol.InventoryInfo{
			{Type: 2, ID: 23001, Stock: 275737, Count: 28, Received: 28},
		},
	})
	if got := s.GetInventory(2, 23001); got != 275737 {
		t.Fatalf("库存=%d want 275737（未登记的响应也该被通用层折叠）", got)
	}
}

// 按 Stock 覆盖而非累加 Count——这是真机数据定的案：同一 id 的多条奖励里 Stock 是同一个
// "结算后最终余额"，Count 才逐条不同。累加 Count 会让库存翻倍。
func TestFoldRewardsOverwritesWithStock(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	// 取自真机样本的形态：同一 id 两条，Stock 相同、Count 不同。
	r.Apply(s, nil, &daily.PresentReceiveAllResponse{
		Rewards: []protocol.InventoryInfo{
			{Type: 12, ID: 94002, Stock: 868883605, Count: 16000, Received: 16000},
			{Type: 12, ID: 94002, Stock: 868883605, Count: 40000, Received: 40000},
		},
	})
	if got := s.Gold.Free; got != 868883605 {
		t.Fatalf("金币=%d want 868883605（Stock 覆盖；若累加 Count 会得到别的数）", got)
	}
}

// 金币/钻石不进库存表，与 GetInventory 的特判对称——漏了这条，模块读到的余额恒为 0。
func TestFoldRewardsRoutesCurrencies(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &daily.PresentReceiveAllResponse{
		Rewards: []protocol.InventoryInfo{
			{Type: 12, ID: 94002, Stock: 500, Count: 1},   // mana
			{Type: 8, ID: 91002, Stock: 71722, Count: 20}, // jewel
			{Type: 2, ID: 22001, Stock: 2785, Count: 3},   // 普通物品
		},
	})
	if s.Gold.Free != 500 {
		t.Errorf("金币未落到专用字段: %+v", s.Gold)
	}
	if s.Jewel.Free != 71722 {
		t.Errorf("钻石未落到专用字段: %+v", s.Jewel)
	}
	if _, ok := s.Inventory[gamestate.InventoryKey{Type: 12, ID: 94002}]; ok {
		t.Error("金币不该进库存表")
	}
	if _, ok := s.Inventory[gamestate.InventoryKey{Type: 8, ID: 91002}]; ok {
		t.Error("钻石不该进库存表")
	}
	if s.Inventory[gamestate.InventoryKey{Type: 2, ID: 22001}] != 2785 {
		t.Errorf("普通物品未进库存表: %+v", s.Inventory)
	}
}

// seasonpass 是唯一逆序的那个（照搬参考项目 handlers.py:900 的 rewards[::-1]）。若逐条 stock 是
// 累进中间值，正序遍历会让最旧的一条胜出——这里用不同 stock 的两条把顺序钉死。
func TestSeasonpassRewardsFoldInReverse(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &seasonpass.MissionAcceptResponse{
		Rewards: []protocol.InventoryInfo{
			{Type: 2, ID: 20001, Stock: 300}, // 服务端排在前 → 应最终胜出
			{Type: 2, ID: 20001, Stock: 100},
		},
	})
	if got := s.GetInventory(2, 20001); got != 300 {
		t.Fatalf("库存=%d want 300（逆序折叠应让 rewards[0] 最后写入）", got)
	}
}

// 赛马抽取'不实现'RewardCarrier——参考项目的 handlers.py 里该响应根本没有 handler。
// 无样本又无参考项目依据时不折，比猜一个语义安全。
func TestCharaFortuneDrawCarriesNoRewards(t *testing.T) {
	var resp any = &race.CharaFortuneDrawResponse{}
	if _, ok := resp.(protocol.RewardCarrier); ok {
		t.Fatal("赛马抽取不该实现 RewardCarrier：ref 无对应 handler，且本库无真机样本")
	}
}

// 体力快照必须折回：不折就是'状态过期'而非报错——领取类模块跑完后，模块读到的还是登录时
// 的旧体力。参考项目的对应 handler 都折了（clan/like、mission/accept、present、room 四处）。
func TestStaminaSnapshotFolds(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	s.Stamina = 10

	r.Apply(s, nil, &clan.ClanLikeResponse{
		StaminaInfo: &protocol.UserStaminaInfo{UserStamina: 137, StaminaFullRecoveryTime: 1754400000},
	})
	if s.Stamina != 137 {
		t.Errorf("体力=%d want 137（通用层未折体力快照）", s.Stamina)
	}
	if s.StaminaFullRecoveryTime != 1754400000 {
		t.Errorf("回满时刻=%d，需与体力同源折叠", s.StaminaFullRecoveryTime)
	}
	// 同一响应还要走域专属层——通用与专属两层都作用于它，互不吞噬。
	if s.ClanLikeCount != 1 {
		t.Error("域专属折叠被通用层挤掉了")
	}
}

// 余额快照同理（参考项目的 mirage/receive_reward 折 user_gold + user_jewel）。
func TestBalanceSnapshotFolds(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &mirage.ReceiveRewardResponse{
		UserGold:  &protocol.UserGold{GoldIDFree: 900, GoldIDPay: 100},
		UserJewel: &protocol.UserJewel{FreeJewel: 300, Jewel: 50},
	})
	if s.Gold.Free != 900 || s.Gold.Paid != 100 {
		t.Errorf("金币未折: %+v", s.Gold)
	}
	if s.Jewel.Free != 300 || s.Jewel.Paid != 50 {
		t.Errorf("钻石未折: %+v", s.Jewel)
	}
}

// team_level 走域专属层（裸 int 没有共用模型可依附）。参考项目用 `if self.team_level` 守卫：
// 服务端不下发时是 0，无条件赋值会把等级抹掉。
func TestMissionAcceptKeepsTeamLevelWhenAbsent(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	s.TeamLevel = 200

	r.Apply(s, nil, &daily.MissionAcceptResponse{}) // 不下发 team_level
	if s.TeamLevel != 200 {
		t.Fatalf("等级=%d，缺席的 team_level 不该把已有等级抹成 0", s.TeamLevel)
	}
	r.Apply(s, nil, &daily.MissionAcceptResponse{TeamLevel: 201})
	if s.TeamLevel != 201 {
		t.Fatalf("等级=%d want 201", s.TeamLevel)
	}
}

// clan/info 与 load/index 折的是同一个 ClanID，保持同源——哪个响应回传就以哪个为准。
func TestClanInfoFoldsClanID(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &clan.ClanInfoResponse{Clan: &clan.ClanData{
		Detail: &clan.ClanDetail{ClanID: 4242},
	}})
	if s.ClanID != 4242 {
		t.Fatalf("ClanID=%d want 4242", s.ClanID)
	}
	// detail 缺席时不该把已有公会抹掉。
	r.Apply(s, nil, &clan.ClanInfoResponse{Clan: &clan.ClanData{}})
	if s.ClanID != 4242 {
		t.Fatalf("ClanID=%d，detail 缺席不该抹掉已有值", s.ClanID)
	}
}

func TestMissionIndexFoldsMissions(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &daily.MissionIndexResponse{Missions: []daily.UserMissionInfo{
		{MissionID: 11, MissionStatus: 2},
		{MissionID: 22, MissionStatus: 3},
	}})
	if s.Missions[11] != 2 || s.Missions[22] != 3 {
		t.Fatalf("missions 折叠错误: %+v", s.Missions)
	}
}

func TestApplyFoldsLoadIndex(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &account.LoadIndexResponse{
		UserInfo:  &account.UserInfo{ViewerID: 123, UserName: "骑士君", TeamLevel: 200, UserStamina: 50},
		UserJewel: &protocol.UserJewel{Jewel: 1000, FreeJewel: 300},
		UserGold:  &protocol.UserGold{GoldIDPay: 100, GoldIDFree: 900},
	})
	if s.UserName != "骑士君" || s.ViewerID != 123 || s.TeamLevel != 200 || s.Stamina != 50 {
		t.Fatalf("user_info 未正确折叠: %+v", s)
	}
	if s.Jewel.Paid != 1000 || s.Jewel.Free != 300 {
		t.Fatalf("jewel 折叠错误: %+v", s.Jewel)
	}
	if s.Gold.Total() != 1000 {
		t.Fatalf("gold=%d want 1000", s.Gold.Total())
	}
}

func TestApplyFoldsMaintenance(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &sdk.SourceIniGetMaintenanceStatusResponse{ResVer: "10002200", ManifestVer: "abc"})
	if s.ResVer != "10002200" || s.ManifestVer != "abc" {
		t.Fatalf("maintenance 折叠错误: %+v", s)
	}
}

func TestApplyIgnoresUnregistered(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	// 未登记类型：不应 panic，也不应改动状态。
	r.Apply(s, nil, &sdk.CheckGameStartResponse{NowTutorial: true})
	if s.UserName != "" || s.ViewerID != 0 {
		t.Fatalf("未登记类型不应改动状态: %+v", s)
	}
}

// exec 回传的 user_gold 必须折回：模块每发都带 current_gold 快照，漏折则第二发起就是过期值。
func TestApplyFoldsAlcesExecGold(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	s.Gold = gamestate.Currency{Free: 1_000_000}
	s.Inventory = map[gamestate.InventoryKey]int{}
	r.Apply(s, nil, &alces.ExecResponse{
		CurrentAlcesPoint: &protocol.InventoryInfo{Type: 2, ID: 26202, Stock: 7},
		UserGold:          &protocol.UserGold{GoldIDPay: 100_000, GoldIDFree: 800_000},
	})
	if s.Gold.Total() != 900_000 {
		t.Fatalf("gold=%d want 900000", s.Gold.Total())
	}
	if s.Inventory[gamestate.InventoryKey{Type: 2, ID: 26202}] != 7 {
		t.Fatalf("炼成 PT 未折叠: %+v", s.Inventory)
	}
}

// 点赞/赛马抽取都必须回写守卫依据，否则同一会话内重跑会绕过守卫、撞上业务错误码。
func TestApplyFoldsGuardsAfterWrites(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	s.CharaFortune = &gamestate.CharaFortune{FortuneID: 1, UnitID: 100101, Rank: 3}

	r.Apply(s, nil, &clan.ClanLikeResponse{})
	if s.ClanLikeCount != 1 {
		t.Fatalf("ClanLikeCount=%d want 1", s.ClanLikeCount)
	}
	r.Apply(s, nil, &race.CharaFortuneDrawResponse{})
	if s.CharaFortune != nil {
		t.Fatalf("赛马抽取后 CharaFortune 应清空: %+v", s.CharaFortune)
	}
}

// 钻石的 jewel 与 free_jewel 是'互斥的两段'（付费 / 免费），不是"总量与其中的免费部分"。
// 这条单独锁：误读过一次——本地字段一度叫 Jewel.Total 却装着付费部分，展示层于是把付费钻
// 当成总额报了出去。取证见 protocol.UserJewel 的说明。
func TestJewelPaidAndFreeAreDisjoint(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &account.LoadIndexResponse{
		UserJewel: &protocol.UserJewel{Jewel: 1000, FreeJewel: 300},
	})
	if s.Jewel.Paid != 1000 || s.Jewel.Free != 300 {
		t.Fatalf("付费/免费错位: paid=%d free=%d want 1000/300", s.Jewel.Paid, s.Jewel.Free)
	}
	if got := s.Jewel.Total(); got != 1300 {
		t.Fatalf("Total()=%d want 1300——两段互斥，合计是相加而非取 jewel", got)
	}
}

// 登录序列是权威全量源，故重登前要清零。这里锁住最容易漏的一类：折叠器用 if 保护的字段
// ——退会后 load/index 不再下发 user_clan，不清零则 ClanID 停在旧公会上。
func TestResetClearsStaleGuardedFields(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &account.LoadIndexResponse{
		UserInfo: &account.UserInfo{ViewerID: 123, UserName: "骑士君"},
		UserClan: &account.UserClan{ClanID: 999},
	})
	if s.ClanID != 999 {
		t.Fatalf("前置条件不成立，ClanID=%d want 999", s.ClanID)
	}

	// 重登：清零后，退会的账号不再下发 user_clan。
	s.Reset()
	r.Apply(s, nil, &account.LoadIndexResponse{
		UserInfo: &account.UserInfo{ViewerID: 123, UserName: "骑士君"},
	})
	if s.ClanID != 0 {
		t.Fatalf("退会后 ClanID 应清零，实为 %d（陈旧值活过了重登）", s.ClanID)
	}
}

// 模块本轮折叠的本地增量同样不该活过重登——重登后一律以服务端全量为准。
func TestResetClearsModuleFoldedIncrements(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	r.Apply(s, nil, &clan.ClanLikeResponse{}) // 点赞后置 1
	s.Inventory = map[gamestate.InventoryKey]int{{Type: 2, ID: 26202}: 7}
	if s.ClanLikeCount != 1 {
		t.Fatalf("前置条件不成立，ClanLikeCount=%d want 1", s.ClanLikeCount)
	}

	s.Reset()
	if s.ClanLikeCount != 0 || s.Inventory != nil {
		t.Fatalf("Reset 未清干净: like=%d inventory=%v", s.ClanLikeCount, s.Inventory)
	}
}

func TestApplyNilPartialFields(t *testing.T) {
	r := fold.DefaultRegistry()
	s := gamestate.New()
	// user_jewel / user_gold 缺失时不应 panic。
	r.Apply(s, nil, &account.LoadIndexResponse{
		UserInfo: &account.UserInfo{UserName: "无钻玩家"},
	})
	if s.UserName != "无钻玩家" || s.Jewel.Total() != 0 || s.Gold.Total() != 0 {
		t.Fatalf("部分字段缺失处理错误: %+v", s)
	}
}
