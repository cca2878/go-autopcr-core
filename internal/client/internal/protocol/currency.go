package protocol

// 本文件是可分"免费 / 付费"两部分的货币模型。服务端把两者分开记账，扣费时先扣免费部分。
//
// 两个口径不可混用，混了就是线上事故：
//
//	账面合计（Total）  服务端口径。上行快照必须用它——彩装炼成每发都要带 current_gold，
//	                  金额与服务端账面对不上会被直接拒。
//	免费部分（Free）    对玩家有意义的那个数。付费部分是充值来的，自动化不该动它，
//	                  故展示一律默认取免费部分。

// UserGold 是金币（mana）持有量。服务端在几十种响应里都回传它作为余额快照，故放在基包。
type UserGold struct {
	GoldIDPay  int64 `msgpack:"gold_id_pay" json:"gold_id_pay"`
	GoldIDFree int64 `msgpack:"gold_id_free" json:"gold_id_free"`
}

// Total 返回付费+免费合计——服务端账面口径（对应参考项目 get_inventory 的 mana 分支）。
func (g *UserGold) Total() int64 {
	if g == nil {
		return 0
	}
	return g.GoldIDPay + g.GoldIDFree
}

// Free 返回免费部分——展示口径。
func (g *UserGold) Free() int64 {
	if g == nil {
		return 0
	}
	return g.GoldIDFree
}

// UserJewel 是钻石持有量。
//
// jewel 与 free_jewel 是'互斥的两部分'，不是"总量与其中的免费部分"——后者是很容易犯的
// 误读，犯了就会把付费钻当成总额展示。取证见参考项目 pcrclient.py：扣费时先扣 free_jewel、不足
// 再扣 jewel（788-793），且上行的 current_currency_num 传的是两者之和（1450 等处）——若
// jewel 已是总量，这个和会超出账面而被服务端拒。
type UserJewel struct {
	Jewel     int `msgpack:"jewel" json:"jewel"`           // 付费部分
	FreeJewel int `msgpack:"free_jewel" json:"free_jewel"` // 免费部分
}

// Total 返回付费+免费合计——服务端账面口径（对应参考项目 get_inventory 的 jewel 分支）。
func (j *UserJewel) Total() int {
	if j == nil {
		return 0
	}
	return j.Jewel + j.FreeJewel
}

// Free 返回免费部分——展示口径。
func (j *UserJewel) Free() int {
	if j == nil {
		return 0
	}
	return j.FreeJewel
}

// GoldCarrier / JewelCarrier 让折叠层取出响应携带的货币余额快照。
//
// 拆成两个单方法接口而非合并：多数响应只回传其中一种，合并会逼着实现方写一个恒返回 nil
// 的方法。user_gold 是参考项目折叠面里出现频次第二高的模型（40 处），仅次于库存条目。
type GoldCarrier interface {
	// GoldSnapshot 返回本次响应携带的金币余额；未携带时返回 nil。
	GoldSnapshot() *UserGold
}

// JewelCarrier 见 GoldCarrier。
type JewelCarrier interface {
	// JewelSnapshot 返回本次响应携带的钻石余额；未携带时返回 nil。
	JewelSnapshot() *UserJewel
}
