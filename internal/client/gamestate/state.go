// Package gamestate 保存无头客户端观察到的玩家状态，并提供「响应→状态」折叠。
//
// 对应原 Python 项目的 datamgr（玩家数据聚合）与 handlers（各响应的 update 逻辑）。
// M2 先覆盖登录序列可得的基础档案；战力、库存等需要母数据的派生量属后续增量。
//
// 本包是纯数据层：不依赖 transport / 网络，可被上层用 mock 方式独立测试。
package gamestate

import (
	"math"
	"net/url"
)

// Jewel 是钻石信息。
type Jewel struct {
	Total int // 总钻石
	Free  int // 免费钻石
}

// OwnedUnit 是玩家持有的一个角色（练度/图鉴类报告所需字段）。
type OwnedUnit struct {
	ID             int
	Level          int
	PromotionLevel int
	Rarity         int
}

// CharaFortune 是今日赛马待抽取信息（登录时由 load/index 折叠；nil＝今日已赛马或无赛马数据）。
type CharaFortune struct {
	FortuneID int
	UnitID    int // 抽取所选角色（取 cf.unit_list[0]）
	Rank      int // 今日名次
}

// InventoryKey 是库存物品的 (类型, id) 键（对应 ref ItemType＝(eInventoryType, id)）。
type InventoryKey struct {
	Type int // eInventoryType
	ID   int
}

// 特殊货币的库存键（对应 ref db.zmana / db.mana / db.jewel）。这三者不进 Inventory 表，
// 而由 load/index 折进 PlayerState.Gold / Jewel 专用字段，故 GetInventory 需特判。
var (
	keyZMana = InventoryKey{Type: 12, ID: 94000} // (eInventoryType.Gold, 94000)
	keyMana  = InventoryKey{Type: 12, ID: 94002} // (eInventoryType.Gold, 94002)
	keyJewel = InventoryKey{Type: 8, ID: 91002}  // (eInventoryType.Jewel, 91002)
)

// ExEquipSubStatus 是一件 EX 装备的一条副属性（登录/炼成响应折叠而来）。
// Status＝属性类型(eParamType)，Step＝档位(1..5，5＝满级)，IsLock＝是否锁定该槽。
type ExEquipSubStatus struct {
	SlotNumber int
	Status     int
	Step       int
	IsLock     bool
}

// ExEquip 是玩家持有的一件 EX 装备的完整实例（登录时由 load/index 折叠）。彩装究极炼成、
// EX 装战力搭配等需要 serial_id/rank/enhancement_pt/sub_status，故保留完整实例而非仅 id。
type ExEquip struct {
	SerialID       int
	ExEquipmentID  int
	EnhancementPt  int
	Rank           int
	ProtectionFlag int
	SubStatus      []ExEquipSubStatus
	IsAlcesPending bool
}

// PlayerState 是无头客户端聚合的玩家状态。
type PlayerState struct {
	ViewerID  int64
	UserName  string
	TeamLevel int
	Stamina   int
	Jewel     Jewel
	Gold      int64

	// 公会相关（登录时由 load/index 折叠而来）。
	ClanID        int64 // 所属公会 id；未加入公会为 0
	ClanLikeCount int   // 今日已用点赞次数；>0 表示今日已点赞

	// DailyResetTime 是本会话的失效时刻（Unix 秒，登录时由 load/index 折叠）：服务端在每日
	// 重置点丢弃会话，越过它再发请求必被判失效。0＝尚未登录/服务端未下发。
	DailyResetTime int64

	// ReadStoryIDs 是已阅读的剧情 id 列表（登录时由 load/index 折叠而来）。
	ReadStoryIDs []int

	// 任务通关状态（登录时由 home/index 折叠而来），供剧情等解锁门禁判定。
	ClearedQuests      map[int]struct{} // 已通关的普通任务 id（clear_flg>0）
	ClearedBywayQuests map[int]struct{} // 已通关的支线任务 id

	// UnitLove 是各角色的好感等级（chara_id→love_level，登录时由 load/index 折叠）。
	// 未持有的角色不在表中。供角色好感剧情解锁判定。
	UnitLove map[int]int

	// CharaLove 是各角色的累计亲密度（chara_id→chara_love，登录时由 load/index 折叠），
	// 供喂蛋糕判定亲密度是否已满。未持有的角色不在表中。
	CharaLove map[int]int

	// Units 是玩家持有的角色列表（登录时由 load/index 折叠），供练度/图鉴类报告。
	Units []OwnedUnit

	// CharaFortune 是今日赛马待抽取信息（登录时由 load/index 折叠）；nil＝今日已赛马/无数据。
	CharaFortune *CharaFortune

	// ExEquipIDs 是玩家持有的 EX 装备 ex_equipment_id 列表（登录时由 load/index 折叠），供计数/图鉴报告。
	// 与 ExEquips 同源折叠，保留以兼容只需 id 的旧调用（如查ex装备计数）。
	ExEquipIDs []int

	// ExEquips 是玩家持有的 EX 装备完整实例（serial_id→实例，登录时由 load/index 折叠、炼成响应
	// 增量更新），供彩装炼成/战力搭配。按 serial_id 键以便炼成定案/锁定按序更新（对应 ref ex_equips dict）。
	ExEquips map[int]ExEquip

	// Inventory 是普通库存物品持有量（(类型,id)→stock，登录时由 load/index 的 item_list +
	// material_list 折叠），供 get_inventory 查询（如彩装炼成 PT、炼成材料）。经 GetInventory 读取。
	Inventory map[InventoryKey]int

	// 训练（探索）扫荡次数（登录时由 home/index 折叠），供 EXP/Mana 探索扫荡量报告。
	TrainingExpDone  int // 今日已用 exp 探索次数
	TrainingExpMax   int // exp 探索每日上限
	TrainingManaDone int // 今日已用 mana(gold) 探索次数
	TrainingManaMax  int // mana 探索每日上限

	// 版本信息（登录时由维护状态响应折叠而来）。
	ResVer      string
	ManifestVer string

	// ResURLs 是维护状态响应下发的【全部】资源 CDN 根（按 res_http_type 定 scheme），顺序即
	// 下发顺序。供 masterdata 在线获取使用：首台故障时依次换用下一台（见 asset.Source）。
	// 下发为空/全部非法时为空，由上层回退到内置默认 CDN。
	ResURLs []*url.URL
}

// ReadStorySet 返回已读剧情 id 的集合（含哨兵 0＝无前置），便于成员判定。
func (s *PlayerState) ReadStorySet() map[int]struct{} {
	set := make(map[int]struct{}, len(s.ReadStoryIDs)+1)
	set[0] = struct{}{} // 0 表示"无前置剧情"，恒视为已读
	for _, id := range s.ReadStoryIDs {
		set[id] = struct{}{}
	}
	return set
}

// OwnedUnitIDs 返回持有角色 id 集合（供图鉴缺口判定）。
func (s *PlayerState) OwnedUnitIDs() map[int]struct{} {
	set := make(map[int]struct{}, len(s.Units))
	for _, u := range s.Units {
		set[u.ID] = struct{}{}
	}
	return set
}

// IsQuestCleared 报告某普通任务是否已通关（供竞技场等解锁门禁）。
func (s *PlayerState) IsQuestCleared(questID int) bool {
	_, ok := s.ClearedQuests[questID]
	return ok
}

// IsQuestUnlocked 报告某任务是否已通关（供剧情解锁门禁）。复刻 ref unlock_quest_id 的普通/支线
// 分支：quest==0（无门禁）、已通关普通任务、已通关支线任务。露娜塔分支暂未跟踪（塔剧情才需）。
func (s *PlayerState) IsQuestUnlocked(questID int) bool {
	if questID == 0 {
		return true
	}
	if _, ok := s.ClearedQuests[questID]; ok {
		return true
	}
	_, ok := s.ClearedBywayQuests[questID]
	return ok
}

// GetInventory 返回某库存物品 (类型,id) 的持有量（不在库存中＝0）。复刻 ref get_inventory：
// mana/zmana 与 jewel 不在库存表里，需从 Gold / Jewel 专用字段取（如彩装炼成的 mana 消耗）。
func (s *PlayerState) GetInventory(typ, id int) int {
	switch (InventoryKey{Type: typ, ID: id}) {
	case keyZMana, keyMana:
		return clampToInt(s.Gold)
	case keyJewel:
		return s.Jewel.Total + s.Jewel.Free
	}
	return s.Inventory[InventoryKey{Type: typ, ID: id}]
}

// clampToInt 把 int64 收敛进 int，避免 32 位平台（gomobile arm32）上的截断为负。
func clampToInt(v int64) int {
	if v > int64(math.MaxInt) {
		return math.MaxInt
	}
	if v < int64(math.MinInt) {
		return math.MinInt
	}
	return int(v)
}

// New 返回一个空的 PlayerState。
func New() *PlayerState { return &PlayerState{} }
