package labyrinth

import (
	"testing"

	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// linearArea1 造一条区域1 单链地图（col1..6，各 row1，id＝列号），各列类型由 types 给定。
func linearArea1(types [6]int) []lab.Block {
	bs := make([]lab.Block, 6)
	for i := range 6 {
		var next []int
		if i < 5 {
			next = []int{i + 2}
		}
		bs[i] = lab.Block{Area: 1, Column: i + 1, Row: 1, BlockID: i + 1, BlockType: types[i], NextBlockIDList: next}
	}
	return bs
}

// area1 完美模板类型：[起点1, 普通怪2, 角色4, 普通怪2, 角色4, 遗物6]。
var area1Perfect = [6]int{1, 2, 4, 2, 4, 6}

func TestFindAreaRoute_PerfectMatch(t *testing.T) {
	f := &finder{perfectStart: true}
	route, reason := f.findAreaRoute(1, linearArea1(area1Perfect))
	if route == nil {
		t.Fatalf("完美模板应找到路线，得失败：%s", reason)
	}
	if len(route) != 6 || route[0].Column != 1 || route[5].Column != 6 {
		t.Fatalf("路线应贯穿 col1..6，得 %d 格", len(route))
	}
}

func TestFindAreaRoute_PerfectMismatch(t *testing.T) {
	types := area1Perfect
	types[2] = 5 // col3 应为角色(4)，改成事件(5) → 完美开局无路线
	f := &finder{perfectStart: true}
	if route, _ := f.findAreaRoute(1, linearArea1(types)); route != nil {
		t.Fatalf("列类型不符模板，完美开局不应有路线，得 %v", route)
	}
}

func TestFindAreaRoute_NonPerfectIgnoresTemplate(t *testing.T) {
	types := area1Perfect
	types[2] = 5 // 类型不符，但非完美开局只要可达即可
	f := &finder{perfectStart: false}
	if route, reason := f.findAreaRoute(1, linearArea1(types)); route == nil {
		t.Fatalf("非完美开局应找到任意可达路线，得失败：%s", reason)
	}
}

// TestDFS_BranchBacktrack 验证分支回溯：col1 分叉到两个 col2，仅一条分支后续吻合模板。
func TestDFS_BranchBacktrack(t *testing.T) {
	// 主链 col1→(2a 类型不符 / 2b 类型符)→...→col6。仅经 2b 的路线满足完美模板。
	blocks := []lab.Block{
		{Area: 1, Column: 1, Row: 1, BlockID: 1, BlockType: 1, NextBlockIDList: []int{21, 22}},
		{Area: 1, Column: 2, Row: 1, BlockID: 21, BlockType: 5, NextBlockIDList: []int{3}}, // 类型不符(应为2)
		{Area: 1, Column: 2, Row: 2, BlockID: 22, BlockType: 2, NextBlockIDList: []int{3}}, // 类型符
		{Area: 1, Column: 3, Row: 1, BlockID: 3, BlockType: 4, NextBlockIDList: []int{4}},
		{Area: 1, Column: 4, Row: 1, BlockID: 4, BlockType: 2, NextBlockIDList: []int{5}},
		{Area: 1, Column: 5, Row: 1, BlockID: 5, BlockType: 4, NextBlockIDList: []int{6}},
		{Area: 1, Column: 6, Row: 1, BlockID: 6, BlockType: 6, NextBlockIDList: nil},
	}
	f := &finder{perfectStart: true}
	route, reason := f.findAreaRoute(1, blocks)
	if route == nil {
		t.Fatalf("应经 col2 符合项找到路线，得失败：%s", reason)
	}
	if route[1].BlockID != 22 {
		t.Fatalf("应选类型吻合的 col2 分支(22)，得 %d", route[1].BlockID)
	}
}

func TestFindAreaRoute_MissingData(t *testing.T) {
	f := &finder{}
	if _, reason := f.findAreaRoute(1, nil); reason != "区域1没有地图数据" {
		t.Fatalf("空地图应报无数据，得 %q", reason)
	}
}

func TestBossMatches(t *testing.T) {
	f := &finder{
		bossByQuest: map[int][]int{999: {312505}, 888: {303306}},
		area3Bosses: map[int]bool{312505: true}, // 只选 厄勒克特拉夫人
	}
	if !f.bossMatches(3, lab.Block{QuestID: 999}) {
		t.Fatal("命中所选 Boss 应为 true")
	}
	if f.bossMatches(3, lab.Block{QuestID: 888}) {
		t.Fatal("未命中所选 Boss 应为 false")
	}
	// 空选择＝不约束。
	if !(&finder{}).bossMatches(3, lab.Block{QuestID: 888}) {
		t.Fatal("空选择应不约束(true)")
	}
}

func TestExpectedBlockTypes_ThirdBlock(t *testing.T) {
	cases := []struct {
		third string
		want  map[int]bool
	}{
		{"必须事件", map[int]bool{5: true}},
		{"两者都行", map[int]bool{5: true, 6: true}},
		{"必须遗物", map[int]bool{6: true}},
	}
	for _, c := range cases {
		f := &finder{thirdBlockType: c.third}
		got := f.expectedBlockTypes(3, 3) // 区域3 第3格
		if len(got) != len(c.want) {
			t.Fatalf("%s：期望 %v，得 %v", c.third, c.want, got)
		}
		for k := range c.want {
			if !got[k] {
				t.Fatalf("%s：缺类型 %d", c.third, k)
			}
		}
	}
	// 非 3/5 区第3格走模板：区域1 第3列＝角色(4)。
	if !(&finder{}).expectedBlockTypes(1, 3)[4] {
		t.Fatal("区域1第3列应期望类型4(角色)")
	}
}

func TestMaxUnlockedDifficulty(t *testing.T) {
	if got := maxUnlockedDifficulty(&lab.TopResult{}); got != 1 {
		t.Fatalf("无通关记录应为 1，得 %d", got)
	}
	if got := maxUnlockedDifficulty(&lab.TopResult{ClearedDifficulties: []int{1, 2}}); got != 3 {
		t.Fatalf("已通关到2应解锁3，得 %d", got)
	}
	if got := maxUnlockedDifficulty(&lab.TopResult{ClearedDifficulties: []int{5}}); got != 5 {
		t.Fatalf("已通关5应封顶5，得 %d", got)
	}
}

func TestPositionName(t *testing.T) {
	// 某列 3 行：row1下 row2中 row3上。
	col := map[int][]lab.Block{2: {{Column: 2, Row: 1}, {Column: 2, Row: 2}, {Column: 2, Row: 3}}}
	if got := positionName(lab.Block{Column: 2, Row: 2}, col); got != "中" {
		t.Fatalf("3行中 row2 应为 中，得 %q", got)
	}
	// 单行＝合流。
	one := map[int][]lab.Block{1: {{Column: 1, Row: 1}}}
	if got := positionName(lab.Block{Column: 1, Row: 1}, one); got != "合流" {
		t.Fatalf("单行应为 合流，得 %q", got)
	}
}
