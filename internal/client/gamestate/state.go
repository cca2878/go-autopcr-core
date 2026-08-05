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

// Currency 是可分「免费 / 付费」两部分的货币持有量。钻石与金币同形，故共用一个类型。
//
// 两个口径不可混用：
//
//	Total()  账面合计，服务端口径。上行快照必须用它——彩装炼成每发都要带 current_gold，
//	         金额与服务端账面对不上会被拒。
//	Free     免费部分，展示口径。付费部分是充值来的，自动化不该动它，故展示默认取这个。
//
// 服务端把这两部分分开记账（钻石的 jewel / free_jewel 是互斥的两段，不是「总量与其中的
// 免费部分」，见 protocol.UserJewel 的取证），扣费时先扣免费部分。
type Currency struct {
	Free int64 // 免费部分
	Paid int64 // 付费部分（充值/购买而来）
}

// Total 返回账面合计——服务端口径。
func (c Currency) Total() int64 { return c.Free + c.Paid }

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
	Jewel     Currency
	Gold      Currency

	// StaminaFullRecoveryTime 是体力回满的时刻（Unix 秒）。与 Stamina 同源折叠，因为恢复中的
	// 体力要靠它与当前时刻推算——只存 Stamina 会在恢复过程中读到过期值。推算本身尚未实现
	// （需要 team_info 母数据给出上限），字段先存着。
	StaminaFullRecoveryTime int64

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

	// Missions 是任务完成状态（mission_id→mission_status，由 mission/index 折叠）。
	// 与本包多数字段不同，它目前【没有消费者】——ref 存它是为了 is_mission_finished(system_id)
	// 那类查询（datamgr.py:624），对应模块尚未移植。放在这里是为了让状态面与 ref 对齐，
	// 同预建的那批协议模型一样，等移植到时直接可用。
	Missions map[int]int

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
		return clampToInt(s.Gold.Total())
	case keyJewel:
		return clampToInt(s.Jewel.Total())
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

// Reset 把状态清回零值，供登录序列开始【之前】调用。
//
// 登录序列（maintenance + load/index + home/index）是权威的全量数据源，真实客户端也是这么
// 用的：它下发什么，玩家状态就该是什么。不清零会让两类陈旧值活过重登——
//
//	① 折叠器用 if 保护的字段：服务端本轮不下发即保留旧值。退会后 load/index 不带 user_clan，
//	   ClanID 就会停在旧公会上（见 foldLoadIndex）。
//	② 模块本轮折叠的本地增量：那是基于旧世界的推断（如点赞后置 1 的 ClanLikeCount），
//	   重登后一律以服务端全量为准。
//
// 保留其一而非全清，得到的是「半旧半新」——比整体过期更难排查。
//
// 原地清零而非换新实例：折叠中间件在装配时捕获了本指针（见 client.New），换实例会让后续
// 折叠写进一个没人读的旧对象。
func (s *PlayerState) Reset() { *s = PlayerState{} }
