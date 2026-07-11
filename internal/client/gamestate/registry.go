package gamestate

import (
	"reflect"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/discovery"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
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
	return r
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
	for i, e := range lr.UserExEquip {
		exIDs[i] = e.ExEquipmentID
	}
	s.ExEquipIDs = exIDs
}

func foldMaintenance(s *PlayerState, resp any) {
	m := resp.(*sdk.SourceIniGetMaintenanceStatusResponse)
	s.ResVer = m.ResVer
	s.ManifestVer = m.ManifestVer
	// res CDN 根的解析规则归 discovery（服务端发现域）所有，此处只做折叠。
	s.ResURL = discovery.ResolveResURL(m.ResHTTPType, m.Resource)
}
