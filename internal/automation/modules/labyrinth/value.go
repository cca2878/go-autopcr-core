package labyrinth

import (
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// 本文件是「刷开局 v2」的【按价值判定】算法：不再逐列比对固定模板，而是求路线能取到的
// 贵重格数量，与这张图本身的上界比较。与网络无关，可独立单测。
//
// 为什么换掉模板：模板把「要什么」和「它在第几列」焊死在一起，可生成器会把同一种格子放到
// 不同列（实测：区域4 的第二个 EX 怪落在第5列或第6列各半）。模板写死一列，另一半地图就
// 无论怎么走都判死。按价值判定只问「这条路线取到了几个贵重格」，位置无关，那类错配从源头
// 消失。
//
// 上界【由地图自身算出】，不引入任何经验模型：一条路线每列只能取一格，所以上界＝含贵重格
// 的列数。core 因此不需要知道任何分布数据——「这组条件要刷多少次」是外壳/文档的事。

// 格子类型（与 blockTypeName 同源，给判定逻辑用具名常量）。
const (
	blockStart  = 1
	blockNormal = 2
	blockEx     = 3
	blockChar   = 4
	blockEvent  = 5
	blockRelic  = 6
	blockShop   = 7
	blockBoss   = 8
)

// baseValuable 是默认算作「贵重」的格子类型：值得为之绕路的那些。
// 起点/普通怪/事件是充数格——事件可由偏好提升为贵重（见 pref）。
var baseValuable = map[int]bool{
	blockEx:    true,
	blockChar:  true,
	blockRelic: true,
	blockShop:  true,
}

// pref 是一条「同列二选一」偏好：某一列同时出现 a 与 b 两种贵重格时，只有 winner 算贵重。
//
// 【不写列号】是有意的。三对竞争关系各自只出现在特定列（角色⇄遗物、遗物⇄商店、遗物⇄事件），
// 所以「当它们同列相遇时优先谁」与「在第几列优先谁」等价，却不会因生成器挪动位置而失效。
// area 为 0 表示不限区域。winner 为 0 表示「都行」——两者都算贵重，由连通性自行取舍。
type pref struct {
	area   int
	a, b   int
	winner int
}

// applies 报告该偏好是否作用于此区域。
func (p pref) applies(area int) bool { return p.area == 0 || p.area == area }

// valuer 按价值判定一张地图。Boss 是硬过滤，不参与计分——打不过就是打不过，不该被分数换掉。
type valuer struct {
	prefs       []pref
	bossByQuest map[int][]int
	area3Bosses map[int]bool // 空＝不约束
	area5Bosses map[int]bool
}

// columnValuable 算出某区每一列里【算作贵重】的类型集合。
//
// 从该列实际出现的类型出发（而非全表），偏好才好谈「同列相遇」：两者都在场时才需要取舍。
func (v *valuer) columnValuable(area int, blocks []lab.Block) map[int]map[int]bool {
	present := map[int]map[int]bool{}
	for _, b := range blocks {
		if b.Area != area {
			continue
		}
		if present[b.Column] == nil {
			present[b.Column] = map[int]bool{}
		}
		present[b.Column][b.BlockType] = true
	}

	out := make(map[int]map[int]bool, len(present))
	for col, types := range present {
		val := map[int]bool{}
		for t := range types {
			if baseValuable[t] {
				val[t] = true
			}
		}
		for _, p := range v.prefs {
			if !p.applies(area) || !types[p.a] || !types[p.b] {
				continue // 两者没在同列相遇，这条偏好无从谈起
			}
			switch p.winner {
			case 0: // 都行：两者都算贵重（事件本不在 baseValuable，需显式补上）
				val[p.a], val[p.b] = true, true
			case p.a:
				delete(val, p.b)
				val[p.a] = true
			case p.b:
				delete(val, p.a)
				val[p.b] = true
			}
		}
		out[col] = val
	}
	return out
}

// bound 是这张图在该区的可达上界：含贵重格的列数。路线每列只能取一格，故取不到更多。
func (v *valuer) bound(area int, blocks []lab.Block) int {
	n := 0
	for _, val := range v.columnValuable(area, blocks) {
		if len(val) > 0 {
			n++
		}
	}
	return n
}

// bossOK 报告某 Boss 格是否命中所选 Boss。非 Boss 格恒真；所选集合为空＝不约束。
func (v *valuer) bossOK(area int, b lab.Block) bool {
	if b.BlockType != blockBoss {
		return true
	}
	var selected map[int]bool
	switch area {
	case 3:
		selected = v.area3Bosses
	case 5:
		selected = v.area5Bosses
	}
	if len(selected) == 0 {
		return true
	}
	for _, uid := range v.bossByQuest[b.QuestID] {
		if selected[uid] {
			return true
		}
	}
	return false
}

// unreachable 标记「从此格出发走不到合法终点」，与真实分数区分开。
const unreachable = -1

// best 求该区能取到的最大贵重格数，以及取到它的一条路线。
//
// 列是天然的分层，next 只指向下一列，故这是 DAG 上的最长路——一次记忆化搜索即可，与 v1 的
// 存在性 DFS 同量级。走不到合法终点（末列是 Boss 且不命中所选）时返回 unreachable。
func (v *valuer) best(area int, blocks []lab.Block) (int, []lab.Block) {
	byID := map[int]lab.Block{}
	byColumn := map[int][]lab.Block{}
	last := 0
	for _, b := range blocks {
		if b.Area != area {
			continue
		}
		byID[b.BlockID] = b
		byColumn[b.Column] = append(byColumn[b.Column], b)
		if b.Column > last {
			last = b.Column
		}
	}
	if len(byColumn[1]) == 0 {
		return unreachable, nil
	}
	valuable := v.columnValuable(area, blocks)

	type memoEntry struct {
		score int
		route []lab.Block
	}
	memo := map[int]memoEntry{}
	inProgress := map[int]bool{}

	var walk func(b lab.Block) (int, []lab.Block)
	walk = func(b lab.Block) (int, []lab.Block) {
		if e, ok := memo[b.BlockID]; ok {
			return e.score, e.route
		}
		if inProgress[b.BlockID] { // 数据异常成环时兜底，不让搜索打转
			return unreachable, nil
		}
		inProgress[b.BlockID] = true
		defer delete(inProgress, b.BlockID)

		gain := 0
		if valuable[b.Column][b.BlockType] {
			gain = 1
		}

		score, route := unreachable, []lab.Block(nil)
		if b.Column == last {
			if v.bossOK(area, b) {
				score, route = gain, []lab.Block{b}
			}
		} else {
			for _, nextID := range b.NextBlockIDList {
				nb, ok := byID[nextID]
				if !ok {
					continue
				}
				s, r := walk(nb)
				if s == unreachable || gain+s <= score {
					continue
				}
				score = gain + s
				route = append([]lab.Block{b}, r...)
			}
		}
		memo[b.BlockID] = memoEntry{score: score, route: route}
		return score, route
	}

	bestScore, bestRoute := unreachable, []lab.Block(nil)
	for _, start := range sortedByRow(byColumn[1]) {
		if s, r := walk(start); s > bestScore {
			bestScore, bestRoute = s, r
		}
	}
	return bestScore, bestRoute
}

// judge 判定某区是否达标：可达上界减实得不超过 allowance，且路线合法（Boss 命中）。
// 返回取到的路线、实际少拿的格数。
//
// 【无合法路线时 short 为 unreachable(-1)，不是 0】。0 是「一格没少拿」这个最好的结果，
// 拿它兼表「压根走不通」会把最差的情形报成最好的——遥测里就这么把 418 张 Boss 不命中的
// 地图记成了「完美开局」。调用方一律先判 short >= 0 再当作格数用。
func (v *valuer) judge(area int, blocks []lab.Block, allowance int) (route []lab.Block, short int, ok bool) {
	score, r := v.best(area, blocks)
	if score == unreachable {
		return nil, unreachable, false
	}
	short = v.bound(area, blocks) - score
	return r, short, short <= allowance
}

// sortedByRow 让起点的探索顺序确定，保证同一张图每次都给出同一条路线。
func sortedByRow(blocks []lab.Block) []lab.Block {
	out := append([]lab.Block(nil), blocks...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Row < out[j-1].Row; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
