package gamestate

import (
	"reflect"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/discovery"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
)

// Folder 把一个响应折叠进 PlayerState。resp 为 *R（具体响应类型的指针）。
type Folder func(s *PlayerState, resp any)

// Registry 按响应的具体类型分发到对应 Folder（对应原项目 handlers 的注册表）。
type Registry struct {
	folders map[reflect.Type]Folder
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{folders: make(map[reflect.Type]Folder)}
}

// Register 为 sample 的具体类型登记一个 Folder。sample 传 (*R)(nil) 即可。
func (r *Registry) Register(sample any, f Folder) {
	r.folders[reflect.TypeOf(sample)] = f
}

// Apply 若 resp 的类型已登记，则折叠进 s；否则忽略。
func (r *Registry) Apply(s *PlayerState, resp any) {
	if f, ok := r.folders[reflect.TypeOf(resp)]; ok {
		f(s, resp)
	}
}

// DefaultRegistry 登记 M2 登录序列涉及的折叠器。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register((*account.LoadIndexResponse)(nil), foldLoadIndex)
	r.Register((*account.HomeIndexResponse)(nil), foldHomeIndex)
	r.Register((*sdk.SourceIniGetMaintenanceStatusResponse)(nil), foldMaintenance)
	r.Register((*clan.ClanLikeResponse)(nil), foldClanLike)
	r.Register((*race.CharaFortuneDrawResponse)(nil), foldCharaFortuneDraw)
	r.Register((*alces.ExecResponse)(nil), foldAlcesExec)
	r.Register((*alces.FixResultResponse)(nil), foldAlcesFixResult)
	r.Register((*alces.LockSlotResponse)(nil), foldAlcesLockSlot)
	return r
}

// foldClanLike 记下「今日已点赞」（对应 ref ClanLikeResponse 的 clan_like_count = 1）。
// 不折叠则同一会话里第二次跑点赞模块会绕过守卫、撞上业务错误码。
func foldClanLike(s *PlayerState, _ any) {
	s.ClanLikeCount = 1
}

// foldCharaFortuneDraw 抽完即清空今日赛马待抽信息（对应 ref daily.py 的 client.data.cf = None）。
// 同理：不清则同一会话里重跑赛马模块会绕过「今日已赛马」守卫、重复发抽取请求。
func foldCharaFortuneDraw(s *PlayerState, _ any) {
	s.CharaFortune = nil
}

// subStatusFromAlces 把 alces 协议副属性转为 gamestate 形态。
func subStatusFromAlces(ss []alces.SubStatus) []ExEquipSubStatus {
	out := make([]ExEquipSubStatus, len(ss))
	for i, s := range ss {
		out[i] = ExEquipSubStatus{SlotNumber: s.SlotNumber, Status: s.Status, Step: s.Step, IsLock: s.IsLock}
	}
	return out
}

// foldAlcesExec 把 exec 回传的炼成 PT 余量与金币余额折进状态（pending 副属性尚未定案，不改装备）。
// 金币必须折回：模块每发 exec 都要带 current_gold 快照，漏折则第二发起就是过期值。
func foldAlcesExec(s *PlayerState, resp any) {
	r := resp.(*alces.ExecResponse)
	if r.CurrentAlcesPoint != nil && s.Inventory != nil {
		p := r.CurrentAlcesPoint
		s.Inventory[InventoryKey{Type: p.Type, ID: p.ID}] = p.Stock
	}
	if r.UserGold != nil {
		s.Gold = r.UserGold.Total()
	}
}

// foldAlcesFixResult 用定案回传的完整实例更新对应彩装（副属性/锁定态随之刷新）。
func foldAlcesFixResult(s *PlayerState, resp any) {
	r := resp.(*alces.FixResultResponse)
	if r.FixedAlcesData == nil || s.ExEquips == nil {
		return
	}
	e := r.FixedAlcesData
	s.ExEquips[e.SerialID] = ExEquip{
		SerialID:       e.SerialID,
		ExEquipmentID:  e.ExEquipmentID,
		EnhancementPt:  e.EnhancementPt,
		Rank:           e.Rank,
		ProtectionFlag: e.ProtectionFlag,
		SubStatus:      subStatusFromAlces(e.SubStatus),
		IsAlcesPending: e.IsAlcesPending != 0,
	}
}

// foldAlcesLockSlot 用锁定后的炼成数据列表更新对应彩装的副属性（锁定标志）。
func foldAlcesLockSlot(s *PlayerState, resp any) {
	r := resp.(*alces.LockSlotResponse)
	if s.ExEquips == nil {
		return
	}
	for _, d := range r.AlcesDataList {
		e, ok := s.ExEquips[d.SerialID]
		if !ok {
			continue
		}
		e.SubStatus = subStatusFromAlces(d.SubStatus)
		s.ExEquips[d.SerialID] = e
	}
}

func foldHomeIndex(s *PlayerState, resp any) {
	h := resp.(*account.HomeIndexResponse)
	cleared := make(map[int]struct{})
	for _, q := range h.QuestList {
		if q.ClearFlg > 0 {
			cleared[q.QuestID] = struct{}{}
		}
	}
	s.ClearedQuests = cleared
	byway := make(map[int]struct{}, len(h.ClearedBywayQuestIDList))
	for _, id := range h.ClearedBywayQuestIDList {
		byway[id] = struct{}{}
	}
	s.ClearedBywayQuests = byway
	if h.TrainingQuestCount != nil {
		s.TrainingExpDone = h.TrainingQuestCount.ExpQuest
		s.TrainingManaDone = h.TrainingQuestCount.GoldQuest
	}
	if h.TrainingQuestMaxCount != nil {
		s.TrainingExpMax = h.TrainingQuestMaxCount.ExpQuest
		s.TrainingManaMax = h.TrainingQuestMaxCount.GoldQuest
	}
}

func foldLoadIndex(s *PlayerState, resp any) {
	lr := resp.(*account.LoadIndexResponse)
	if lr.UserInfo != nil {
		s.ViewerID = lr.UserInfo.ViewerID
		s.UserName = lr.UserInfo.UserName
		s.TeamLevel = lr.UserInfo.TeamLevel
		s.Stamina = lr.UserInfo.UserStamina
	}
	if lr.UserJewel != nil {
		s.Jewel = Jewel{Total: lr.UserJewel.Jewel, Free: lr.UserJewel.FreeJewel}
	}
	if lr.UserGold != nil {
		s.Gold = lr.UserGold.GoldIDPay + lr.UserGold.GoldIDFree
	}
	if lr.UserClan != nil {
		s.ClanID = lr.UserClan.ClanID
	}
	s.ClanLikeCount = lr.ClanLikeCount
	s.DailyResetTime = lr.DailyResetTime
	s.ReadStoryIDs = lr.ReadStoryIDs
	love := make(map[int]int, len(lr.UserCharaInfo))
	charaLove := make(map[int]int, len(lr.UserCharaInfo))
	for _, u := range lr.UserCharaInfo {
		love[u.CharaID] = u.LoveLevel
		charaLove[u.CharaID] = u.CharaLove
	}
	s.UnitLove = love
	s.CharaLove = charaLove
	units := make([]OwnedUnit, len(lr.UnitList))
	for i, u := range lr.UnitList {
		units[i] = OwnedUnit{ID: u.ID, Level: u.UnitLevel, PromotionLevel: u.PromotionLevel, Rarity: u.UnitRarity}
	}
	s.Units = units
	if lr.CF != nil && len(lr.CF.UnitList) > 0 {
		s.CharaFortune = &CharaFortune{FortuneID: lr.CF.FortuneID, UnitID: lr.CF.UnitList[0], Rank: lr.CF.Rank}
	} else {
		s.CharaFortune = nil
	}
	exIDs := make([]int, len(lr.UserExEquip))
	exEquips := make(map[int]ExEquip, len(lr.UserExEquip))
	for i, e := range lr.UserExEquip {
		exIDs[i] = e.ExEquipmentID
		subs := make([]ExEquipSubStatus, len(e.SubStatus))
		for j, ss := range e.SubStatus {
			subs[j] = ExEquipSubStatus{SlotNumber: ss.SlotNumber, Status: ss.Status, Step: ss.Step, IsLock: ss.IsLock}
		}
		exEquips[e.SerialID] = ExEquip{
			SerialID:       e.SerialID,
			ExEquipmentID:  e.ExEquipmentID,
			EnhancementPt:  e.EnhancementPt,
			Rank:           e.Rank,
			ProtectionFlag: e.ProtectionFlag,
			SubStatus:      subs,
			IsAlcesPending: e.IsAlcesPending != 0,
		}
	}
	s.ExEquipIDs = exIDs
	s.ExEquips = exEquips

	inv := make(map[InventoryKey]int, len(lr.ItemList)+len(lr.MaterialList))
	for _, it := range lr.ItemList {
		inv[InventoryKey{Type: it.Type, ID: it.ID}] = it.Stock
	}
	for _, it := range lr.MaterialList {
		inv[InventoryKey{Type: it.Type, ID: it.ID}] = it.Stock
	}
	s.Inventory = inv
}

func foldMaintenance(s *PlayerState, resp any) {
	m := resp.(*sdk.SourceIniGetMaintenanceStatusResponse)
	s.ResVer = m.ResVer
	s.ManifestVer = m.ManifestVer
	// res CDN 根的解析规则归 discovery（服务端发现域）所有，此处只做折叠。
	s.ResURLs = discovery.ResolveResURLs(m.ResHTTPType, m.Resource)
}
