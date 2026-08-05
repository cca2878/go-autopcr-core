package protocol

// InventoryInfo 是一条库存/奖励条目——协议里出现频次最高的共用模型（参考项目的 138 个折叠
// handler 里有 144 处读它）。它挂在各域响应的 item_list / material_list / reward_list /
// rewards / reward_info 等字段下，形状始终相同，故定义在基包。
//
// 三个数量字段语义不同、不可互相替代：
//
//	Stock     变动后的余额（服务端账面快照）。load/index 的 item_list、炼成回传的 PT 走这个。
//	Count     本次变动量（增量）。礼物箱领取、家园收取走这个。
//	Received  本次实得量。赛马奖励走这个。
//
// 三个字段通常都给，且 Stock 是本次结算完成后的最终余额——同一 id 的多条奖励里 Stock 相同、
// Count 逐条不同，故折叠只能按 Stock 覆盖，累加 Count 会翻倍。Count 也不可靠：家园收取出现过
// Count=0 而 Received=12 的条目。
type InventoryInfo struct {
	Type     int `msgpack:"type" json:"type"` // eInventoryType
	ID       int `msgpack:"id" json:"id"`
	Stock    int `msgpack:"stock" json:"stock"`
	Count    int `msgpack:"count" json:"count"`
	Received int `msgpack:"received" json:"received"`
}

// RewardCarrier 让折叠层在不知具体响应类型的情况下取出本次的库存变动
// （同 ErrorCarrier 让 transport 取 server_error 的做法）。响应实现本接口即自动被通用层
// 折叠，不必单独注册——参考项目 138 个折叠 handler 里 108 个（78%）都是这种无域专属逻辑、
// 通篇只是把奖励列表灌进库存的形态。
//
// 只给'确认回传 Stock'的响应实现。没有样本佐证的端点先不实现——响应若不带 Stock，通用
// 折叠会把它当成 0 写进库存，把真实持有量冲掉。
type RewardCarrier interface {
	// InventoryChanges 返回本次响应带来的库存变动条目。
	InventoryChanges() []InventoryInfo
}
