package protocol

// 本文件是玩家档案量的共用模型：体力、等级。它们跨域出现的次数在参考项目折叠面里排第 3–4
// （user_stamina_info 17 处、level_info 7 处），且'同一类型挂在不同字段名下'——体力信息
// 在各响应里分别叫 user_stamina_info / stamina_info / user_info。字段名不统一而形状统一，
// 正是该收进基包的信号：域包各写一份的话，同一个东西会有三个名字三份定义。
//
// 这几个模型 core 目前尚未折叠（对应的模块还没移植），先立位置——等折叠落地时直接引用，
// 省掉"先在域包写一份、发现第二处再提取"的那轮返工。

// UserStaminaInfo 是体力状态。恢复中的体力靠 StaminaFullRecoveryTime 与当前时刻推算，
// 故两个字段要一起取，只存 UserStamina 会在恢复过程中读到过期值。
type UserStaminaInfo struct {
	UserStamina             int `msgpack:"user_stamina" json:"user_stamina"`
	StaminaFullRecoveryTime int `msgpack:"stamina_full_recovery_time" json:"stamina_full_recovery_time"`
}

// GaugeInfo 是一条经验/好感槽信息。StartLevel 是本次结算'前'的等级，Total 是累计点数。
type GaugeInfo struct {
	StartLevel int `msgpack:"start_level" json:"start_level"`
	Total      int `msgpack:"total" json:"total"`
	UnitID     int `msgpack:"unit_id" json:"unit_id"`
	CharaID    int `msgpack:"chara_id" json:"chara_id"`
}

// LevelInfo 是一次结算带来的等级变化。参考项目只折叠其中两处：Team.StartLevel 更新主公等级，
// Love 逐条更新角色好感（见 handlers.py:214、365）。
type LevelInfo struct {
	Team *GaugeInfo  `msgpack:"team" json:"team"`
	Unit []GaugeInfo `msgpack:"unit" json:"unit"`
	Love []GaugeInfo `msgpack:"love" json:"love"`
}

// TrainingQuestCount 是训练（探索）关卡的今日次数，按 exp/gold 两类分别计。
type TrainingQuestCount struct {
	GoldQuest int `msgpack:"gold_quest" json:"gold_quest"`
	ExpQuest  int `msgpack:"exp_quest" json:"exp_quest"`
}

// StaminaCarrier 让折叠层在不知具体响应类型的情况下取出体力快照。
//
// 与 RewardCarrier 同构，理由也相同：参考项目里有 17 个响应回传体力，各自字段名还不一样
// （user_stamina_info / stamina_info / user_info）。不折的后果是'状态过期'而非报错——
// 体力只在 load/index 更新的话，跑完领取类模块后读到的就是旧值。
type StaminaCarrier interface {
	// StaminaSnapshot 返回本次响应携带的体力快照；未携带时返回 nil。
	StaminaSnapshot() *UserStaminaInfo
}
