// Package gamestate 保存无头客户端观察到的玩家状态，并提供「响应→状态」折叠。
//
// 对应原 Python 项目的 datamgr（玩家数据聚合）与 handlers（各响应的 update 逻辑）。
// M2 先覆盖登录序列可得的基础档案；战力、库存等需要母数据的派生量属后续增量。
//
// 本包是纯数据层：不依赖 transport / 网络，可被上层用 mock 方式独立测试。
package gamestate

import "net/url"

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
	ExEquipIDs []int

	// 训练（探索）扫荡次数（登录时由 home/index 折叠），供 EXP/Mana 探索扫荡量报告。
	TrainingExpDone  int // 今日已用 exp 探索次数
	TrainingExpMax   int // exp 探索每日上限
	TrainingManaDone int // 今日已用 mana(gold) 探索次数
	TrainingManaMax  int // mana 探索每日上限

	// 版本信息（登录时由维护状态响应折叠而来）。
	ResVer      string
	ManifestVer string

	// ResURL 是维护状态响应下发的资源 CDN 根（取首个主机、按 res_http_type 定 scheme）。
	// 供 masterdata 在线获取使用；下发为空/非法时为 nil，由上层回退到内置默认 CDN。
	ResURL *url.URL
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

// New 返回一个空的 PlayerState。
func New() *PlayerState { return &PlayerState{} }
