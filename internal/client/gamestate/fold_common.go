package gamestate

import "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"

// 本文件是【通用折叠层】：不认响应类型，只认响应携带的共用模型。
//
// 折叠面天然分两层，这是从 ref 的 138 个折叠 handler 统计出来的——其中 108 个（78%）没有
// 任何域专属逻辑，通篇就是把响应里的奖励列表灌进库存、把 user_gold 之类的余额快照抄进状态；
// 真正需要手写域逻辑的只有 22%。按游戏域平铺去拆会把这 78% 打散进十几个文件，且 Gold 这种
// 被上百个响应写的字段会散落各处。故切分面是「通用 / 域专属」，域拆分只在专属层内部做
// （fold_account.go、fold_alces.go…）。

// applyInventory 把一条库存变动写进状态。
//
// 按 Stock 覆盖而非累加 Count：Stock 是本次结算完成后的最终余额，同一 id 的多条奖励里它
// 相同，累加会翻倍（取证见 protocol.InventoryInfo）。
func (s *PlayerState) applyInventory(it protocol.InventoryInfo) {
	key := InventoryKey{Type: it.Type, ID: it.ID}
	switch key {
	case keyZMana, keyMana:
		// 金币/钻石不进库存表，落在专用字段上（与 GetInventory 的特判对称）。
		//
		// 写入【免费部分】依据 ref（update_inventory 把 stock 赋给 gold_id_free /
		// free_jewel）。⚠️ 未能证伪：手头两个测试账号的 gold_id_pay 与 jewel 均为 0，
		// stock 等于免费额还是等于总额在这种账号上无从区分。若将来拿到有付费余额的样本，
		// 这两行是第一个要复核的地方——若 stock 其实是总额，这里会让免费额虚高。
		s.Gold.Free = int64(it.Stock)
	case keyJewel:
		s.Jewel.Free = int64(it.Stock)
	default:
		if s.Inventory == nil {
			s.Inventory = make(map[InventoryKey]int)
		}
		s.Inventory[key] = it.Stock
	}
}

// foldRewards 折叠一个响应携带的全部库存变动。
func foldRewards(s *PlayerState, c protocol.RewardCarrier) {
	for _, it := range c.InventoryChanges() {
		s.applyInventory(it)
	}
}

// foldCommon 跑完整个通用层：响应实现了哪个 Carrier，就折哪一部分。
//
// 这些快照【必须】折回去，否则是状态过期而非报错——体力只在 load/index 更新的话，跑完领取
// 类模块后模块读到的还是登录时的旧值。ref 对应的 handler 都折了（见各 Carrier 的实现处）。
func foldCommon(s *PlayerState, resp any) {
	if c, ok := resp.(protocol.RewardCarrier); ok {
		foldRewards(s, c)
	}
	if c, ok := resp.(protocol.StaminaCarrier); ok {
		if st := c.StaminaSnapshot(); st != nil {
			s.Stamina = st.UserStamina
			s.StaminaFullRecoveryTime = int64(st.StaminaFullRecoveryTime)
		}
	}
	if c, ok := resp.(protocol.GoldCarrier); ok {
		if g := c.GoldSnapshot(); g != nil {
			s.Gold = Currency{Free: g.GoldIDFree, Paid: g.GoldIDPay}
		}
	}
	if c, ok := resp.(protocol.JewelCarrier); ok {
		if j := c.JewelSnapshot(); j != nil {
			s.Jewel = Currency{Free: int64(j.FreeJewel), Paid: int64(j.Jewel)}
		}
	}
}
