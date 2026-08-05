package labyrinth

import (
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// forkedArea 造一张"每列若干行、相邻列全连通"的区域地图。cols[i] 是第 i+1 列各行的类型。
// 全连通是判定测试想要的：这样通过与否只取决于'有没有贵重格'，不掺连通性噪声。
func forkedArea(area int, cols [][]int) []lab.Block {
	id := func(col, row int) int { return area*1000 + col*10 + row }
	var out []lab.Block
	for c, rows := range cols {
		for r, t := range rows {
			var next []int
			if c+1 < len(cols) {
				for r2 := range cols[c+1] {
					next = append(next, id(c+2, r2+1))
				}
			}
			out = append(out, lab.Block{
				Area: area, Column: c + 1, Row: r + 1, BlockID: id(c+1, r+1),
				BlockType: t, NextBlockIDList: next,
			})
		}
	}
	return out
}

// area4Like 造一张形似区域4 的图：第2、7列是 角色/遗物，第3列必有 EX 怪，
// 第二个 EX 怪按 secondExCol 落在第5列或第6列——这正是 v1 模板判死一半地图的地方。
func area4Like(secondExCol int) []lab.Block {
	col5, col6 := []int{blockNormal, blockNormal}, []int{blockNormal, blockNormal}
	switch secondExCol {
	case 5:
		col5 = []int{blockEx, blockNormal}
	case 6:
		col6 = []int{blockEx, blockNormal}
	}
	return forkedArea(4, [][]int{
		{blockStart},
		{blockChar, blockRelic},
		{blockEx, blockNormal},
		{blockEvent, blockEvent},
		col5,
		col6,
		{blockChar, blockRelic},
		{blockShop},
	})
}

// TestSecondExInEitherColumnPasses 是这次重新设计的核心回归：第二个 EX 怪落在第5列还是
// 第6列都应当照样达标。v1 的模板写死"第5列必须是 EX、第6列必须是普通怪"，于是把落在
// 第6列的那一半地图全判死；按价值判定只问拿到几个贵重格，位置无关。
func TestSecondExInEitherColumnPasses(t *testing.T) {
	v := &valuer{}
	for _, col := range []int{5, 6} {
		blocks := area4Like(col)
		route, short, ok := v.judge(4, blocks, 0)
		if !ok {
			t.Fatalf("第二个 EX 在第%d列时应达标，得少拿 %d 格", col, short)
		}
		if route == nil {
			t.Fatalf("第二个 EX 在第%d列：达标却没给出路线", col)
		}
	}

	// 同一张图交给 v1 的模板判定：EX 在第6列时无路线——这就是被修掉的缺陷。
	f := &finder{perfectStart: true}
	if r, _ := f.findAreaRoute(4, area4Like(5)); r == nil {
		t.Fatal("对照组失效：v1 在 EX 位于第5列时本应通过")
	}
	if r, _ := f.findAreaRoute(4, area4Like(6)); r != nil {
		t.Fatal("对照组失效：v1 在 EX 位于第6列时本应判死（否则本测试证明不了什么）")
	}
}

// TestBoundIsColumnsWithValuables 检查上界＝含贵重格的列数：一条路线每列只能取一格。
func TestBoundIsColumnsWithValuables(t *testing.T) {
	v := &valuer{}
	blocks := area4Like(5)
	// 第1列起点、第4列事件、以及不含 EX 的那一列不算；其余 5 列各含贵重格。
	if got := v.bound(4, blocks); got != 5 {
		t.Fatalf("上界应为 5，得 %d", got)
	}
}

// TestAllowanceLetsRouteMissCells 检查"允许少拿几格"确实放宽判定。
func TestAllowanceLetsRouteMissCells(t *testing.T) {
	// 造一张必须二选一的图：第2列上下分叉且此后不再合流，两条支路各只有一个贵重格。
	blocks := []lab.Block{
		{Area: 1, Column: 1, Row: 1, BlockID: 11, BlockType: blockStart, NextBlockIDList: []int{21, 22}},
		{Area: 1, Column: 2, Row: 1, BlockID: 21, BlockType: blockChar, NextBlockIDList: []int{31}},
		{Area: 1, Column: 2, Row: 2, BlockID: 22, BlockType: blockNormal, NextBlockIDList: []int{32}},
		{Area: 1, Column: 3, Row: 1, BlockID: 31, BlockType: blockNormal},
		{Area: 1, Column: 3, Row: 2, BlockID: 32, BlockType: blockRelic},
	}
	v := &valuer{}
	if got := v.bound(1, blocks); got != 2 {
		t.Fatalf("上界应为 2（第2、3列各含贵重格），得 %d", got)
	}
	if _, short, ok := v.judge(1, blocks, 0); ok {
		t.Fatalf("两个贵重格不在同一条路线上，allowance=0 不该通过（少拿 %d）", short)
	}
	if _, _, ok := v.judge(1, blocks, 1); !ok {
		t.Fatal("allowance=1 应当通过")
	}
}

// TestPreferenceNarrowsValuable 检查"同列二选一"偏好：两者同列相遇时只认一个。
//
// 偏好按'类型对'表达、不提列号，所以生成器把这一对挪到别的列也照样生效——这正是不再
// 重蹈模板那种位置耦合的关键。
func TestPreferenceNarrowsValuable(t *testing.T) {
	// 第2列是 角色/遗物 二选一，第3列只有普通怪。
	blocks := forkedArea(1, [][]int{
		{blockStart},
		{blockChar, blockRelic},
		{blockNormal},
	})

	either := &valuer{}
	if got := either.bound(1, blocks); got != 1 {
		t.Fatalf("都行时该列仍只算 1 格上界，得 %d", got)
	}
	if _, _, ok := either.judge(1, blocks, 0); !ok {
		t.Fatal("都行时取任一即达标")
	}

	// 指定只认角色：仍能达标（角色在场），但认定的贵重格变了。
	onlyChar := &valuer{prefs: []pref{{a: blockChar, b: blockRelic, winner: blockChar}}}
	val := onlyChar.columnValuable(1, blocks)
	if !val[2][blockChar] || val[2][blockRelic] {
		t.Fatalf("只认角色时该列贵重集应为 {角色}，得 %v", val[2])
	}

	// 只认商店：本列没有商店，两者都不算贵重 → 该列不计入上界。
	onlyShop := &valuer{prefs: []pref{{a: blockRelic, b: blockShop, winner: blockShop}}}
	if got := onlyShop.bound(1, blocks); got != 1 {
		// 角色仍是贵重（未被这条偏好牵涉），故上界仍为 1。
		t.Fatalf("无关偏好不该改变上界，得 %d", got)
	}
}

// TestPreferenceScopedToArea 检查按区域限定的偏好只作用于该区。
func TestPreferenceScopedToArea(t *testing.T) {
	mk := func(area int) []lab.Block {
		return forkedArea(area, [][]int{{blockStart}, {blockRelic, blockEvent}, {blockNormal}})
	}
	v := &valuer{prefs: []pref{{area: 3, a: blockRelic, b: blockEvent, winner: blockEvent}}}

	if val := v.columnValuable(3, mk(3)); !val[2][blockEvent] || val[2][blockRelic] {
		t.Fatalf("区域3 应只认事件，得 %v", val[2])
	}
	if val := v.columnValuable(5, mk(5)); !val[2][blockRelic] || val[2][blockEvent] {
		t.Fatalf("区域5 不受该偏好影响，应只认遗物（事件本非贵重），得 %v", val[2])
	}
}

// TestBossFilterRejectsWholeMap 检查 Boss 是硬过滤：不命中就整区不可达，而不是扣分。
func TestBossFilterRejectsWholeMap(t *testing.T) {
	blocks := []lab.Block{
		{Area: 3, Column: 1, Row: 1, BlockID: 1, BlockType: blockStart, NextBlockIDList: []int{2}},
		{Area: 3, Column: 2, Row: 1, BlockID: 2, BlockType: blockRelic, NextBlockIDList: []int{3}},
		{Area: 3, Column: 3, Row: 1, BlockID: 3, BlockType: blockBoss, QuestID: 770331601},
	}
	byQuest := map[int][]int{770331601: {301206}}

	hit := &valuer{bossByQuest: byQuest, area3Bosses: map[int]bool{301206: true}}
	if _, _, ok := hit.judge(3, blocks, 0); !ok {
		t.Fatal("命中所选 Boss 时应通过")
	}

	miss := &valuer{bossByQuest: byQuest, area3Bosses: map[int]bool{319604: true}}
	route, short, ok := miss.judge(3, blocks, 0)
	if ok || route != nil {
		t.Fatal("不命中所选 Boss 应整区不可达")
	}
	// 必须是 unreachable 而不是 0：0 是"一格没少拿"这个最好的结果，用它兼表"走不通"，
	// 遥测就会把 Boss 不命中的地图记成完美开局（实测 418/993 张被这么记错）。
	if short != unreachable {
		t.Fatalf("不可达时 short 应为 unreachable(%d)，得 %d", unreachable, short)
	}
	// 放宽 allowance 也救不回来——Boss 不是能用分数换的东西。
	if _, _, ok := miss.judge(3, blocks, 3); ok {
		t.Fatal("allowance 不该能绕过 Boss 过滤")
	}

	empty := &valuer{bossByQuest: byQuest}
	if _, _, ok := empty.judge(3, blocks, 0); !ok {
		t.Fatal("未选任何 Boss＝不约束，应通过")
	}
}

// TestBestIsDeterministic 检查同一张图重复判定给出同一条路线——日志与遥测才可复现。
func TestBestIsDeterministic(t *testing.T) {
	blocks := area4Like(6)
	v := &valuer{}
	first, _ := v.best(4, blocks)
	for range 20 {
		if got, _ := v.best(4, blocks); got != first {
			t.Fatalf("同图多次判定结果不一致：%d ≠ %d", got, first)
		}
	}
}

// TestRegistryHasBothModules 检查新旧模块并存——对比实验的前提。
func TestRegistryHasBothModules(t *testing.T) {
	r := automation.NewRegistry()
	Register(r)
	for _, want := range []string{"labyrinth_start_reroll", "labyrinth_start_reroll_v2"} {
		if _, ok := r.Get(want); !ok {
			t.Fatalf("注册表缺少模块 %q", want)
		}
	}
}
