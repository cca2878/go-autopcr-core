package protocol

// UnitData 是玩家持有的一个角色。跨域复用（ref 折叠面 18 处），字段名同样不统一——
// 各响应里分别叫 unit_list / unit_data / unit_data_list。
//
// 只声明本库折叠会用到的字段：ref 的同名模型有二十余个字段（技能等级、装备槽、专武、
// 超限阶段…），随练度类模块移植再按需增补，不预抄全量。
type UnitData struct {
	ID             int `msgpack:"id" json:"id"`
	UnitRarity     int `msgpack:"unit_rarity" json:"unit_rarity"`
	UnitLevel      int `msgpack:"unit_level" json:"unit_level"`
	PromotionLevel int `msgpack:"promotion_level" json:"promotion_level"`
}
