package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// 本文件是'通用折叠层'：不认响应类型，只认响应携带的共用模型。
//
// 折叠面天然分两层，这是从参考项目的 138 个折叠 handler 统计出来的——其中 108 个（78%）没有
// 任何域专属逻辑，通篇就是把响应里的奖励列表灌进库存、把 user_gold 之类的余额快照抄进状态；
// 真正需要手写域逻辑的只有 22%。按游戏域平铺去拆会把这 78% 打散进十几个文件，且 Gold 这种
// 被上百个响应写的字段会散落各处。故切分面是"通用 / 域专属"，域拆分只在专属层内部做。

// rewards 折叠一个响应携带的全部库存变动。
func rewards(s *gamestate.PlayerState, c protocol.RewardCarrier) {
	for _, it := range c.InventoryChanges() {
		s.ApplyInventory(it)
	}
}

// common 跑完整个通用层：响应实现了哪个 Carrier，就折哪一部分。
//
// 这些快照'必须'折回去，否则是状态过期而非报错——体力只在 load/index 更新的话，跑完领取
// 类模块后模块读到的还是登录时的旧值。参考项目对应的 handler 都折了（见各 Carrier 的实现处）。
func common(s *gamestate.PlayerState, resp any) {
	if c, ok := resp.(protocol.RewardCarrier); ok {
		rewards(s, c)
	}
	if c, ok := resp.(protocol.StaminaCarrier); ok {
		if st := c.StaminaSnapshot(); st != nil {
			s.Stamina = st.UserStamina
			s.StaminaFullRecoveryTime = int64(st.StaminaFullRecoveryTime)
		}
	}
	if c, ok := resp.(protocol.GoldCarrier); ok {
		if g := c.GoldSnapshot(); g != nil {
			s.Gold = gamestate.Currency{Free: g.GoldIDFree, Paid: g.GoldIDPay}
		}
	}
	if c, ok := resp.(protocol.JewelCarrier); ok {
		if j := c.JewelSnapshot(); j != nil {
			s.Jewel = gamestate.Currency{Free: int64(j.FreeJewel), Paid: int64(j.Jewel)}
		}
	}
}
