package protocol

// QuestResult 是一场战斗/扫荡的结算结果。它同样跨域复用且字段名不统一——各响应里分别叫
// quest_result_list / skip_result_list / crush_reward_list（ref 折叠面里 10 处）。
//
// core 目前尚未折叠它（扫荡类模块只读结果不回写状态），先立位置。AcquiredGoldList 那类
// 需要连带引入 SkipGoldRewardInfo 的字段暂不取——按本包既有惯例，只声明确实要用的字段，
// 未知键由解码器忽略。
type QuestResult struct {
	RewardList      []InventoryInfo `msgpack:"reward_list" json:"reward_list"`
	AcquiredGold    int             `msgpack:"acquired_gold" json:"acquired_gold"`
	AcquiredTeamExp int             `msgpack:"acquired_team_exp" json:"acquired_team_exp"`
}
