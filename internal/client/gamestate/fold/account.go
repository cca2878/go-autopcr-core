package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
)

// 账号域的折叠器：登录序列的两个权威全量响应。
func loadIndex(s *gamestate.PlayerState, _ protocol.Request, resp any) {
	lr := resp.(*account.LoadIndexResponse)
	if lr.UserInfo != nil {
		s.ViewerID = lr.UserInfo.ViewerID
		s.UserName = lr.UserInfo.UserName
		s.TeamLevel = lr.UserInfo.TeamLevel
		s.Stamina = lr.UserInfo.UserStamina
	}
	if lr.UserJewel != nil {
		s.Jewel = gamestate.Currency{Free: int64(lr.UserJewel.FreeJewel), Paid: int64(lr.UserJewel.Jewel)}
	}
	if lr.UserGold != nil {
		s.Gold = gamestate.Currency{Free: lr.UserGold.GoldIDFree, Paid: lr.UserGold.GoldIDPay}
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
	units := make([]gamestate.OwnedUnit, len(lr.UnitList))
	for i, u := range lr.UnitList {
		units[i] = gamestate.OwnedUnit{ID: u.ID, Level: u.UnitLevel, PromotionLevel: u.PromotionLevel, Rarity: u.UnitRarity}
	}
	s.Units = units
	if lr.CF != nil && len(lr.CF.UnitList) > 0 {
		s.CharaFortune = &gamestate.CharaFortune{FortuneID: lr.CF.FortuneID, UnitID: lr.CF.UnitList[0], Rank: lr.CF.Rank}
	} else {
		s.CharaFortune = nil
	}
	exIDs := make([]int, len(lr.UserExEquip))
	exEquips := make(map[int]gamestate.ExEquip, len(lr.UserExEquip))
	for i, e := range lr.UserExEquip {
		exIDs[i] = e.ExEquipmentID
		subs := make([]gamestate.ExEquipSubStatus, len(e.SubStatus))
		for j, ss := range e.SubStatus {
			subs[j] = gamestate.ExEquipSubStatus{SlotNumber: ss.SlotNumber, Status: ss.Status, Step: ss.Step, IsLock: ss.IsLock}
		}
		exEquips[e.SerialID] = gamestate.ExEquip{
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

	inv := make(map[gamestate.InventoryKey]int, len(lr.ItemList)+len(lr.MaterialList))
	for _, it := range lr.ItemList {
		inv[gamestate.InventoryKey{Type: it.Type, ID: it.ID}] = it.Stock
	}
	for _, it := range lr.MaterialList {
		inv[gamestate.InventoryKey{Type: it.Type, ID: it.ID}] = it.Stock
	}
	s.Inventory = inv
}

func homeIndex(s *gamestate.PlayerState, _ protocol.Request, resp any) {
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
