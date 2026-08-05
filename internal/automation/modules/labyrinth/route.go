package labyrinth

import (
	"fmt"
	"sort"
	"strings"

	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// 本文件是"黎明界刷开局"的'纯路线判定算法'：给定一张地图(map_list)与刷取参数，判断各目标区域
// 是否存在满足条件的可达路线（完美开局逐列匹配模板 / Boss 命中 / 第 3 格类型），并格式化输出。
// 与网络/客户端无关，可独立单测（见 route_test.go）。对应参考项目 labyrint.py 中的路线判定部分。

// blockTypeName 是格子类型名（对应参考项目 LABYRINTH_BLOCK_TYPE_NAME）。
var blockTypeName = map[int]string{
	1: "起点", 2: "普通怪物", 3: "EX怪物", 4: "角色", 5: "事件", 6: "遗物", 7: "商店", 8: "Boss",
}

// areaRequirements 是各区域每列的期望格子类型（完美开局模板；对应参考项目 AREA_REQUIREMENTS）。
var areaRequirements = map[int]map[int]int{
	1: {1: 1, 2: 2, 3: 4, 4: 2, 5: 4, 6: 6},
	2: {1: 1, 2: 4, 3: 2, 4: 6, 5: 3, 6: 4, 7: 6},
	3: {1: 1, 2: 2, 3: 6, 4: 4, 5: 3, 6: 7, 7: 8},
	4: {1: 1, 2: 4, 3: 3, 4: 5, 5: 3, 6: 2, 7: 4, 8: 7},
	5: {1: 1, 2: 2, 3: 6, 4: 3, 5: 6, 6: 3, 7: 7, 8: 8},
}

// bossNameByUnit 是 unit_id→Boss 名（两区合并；对应参考项目 LABYRINTH_BOSS_NAME_BY_UNIT）。
var bossNameByUnit = func() map[int]string {
	m := map[int]string{}
	for _, b := range append(append([]bossInfo{}, area3Bosses...), area5Bosses...) {
		m[b.unitID] = b.name
	}
	return m
}()

// finder 承载一次刷取的路线判定参数（纯数据，无客户端依赖）。
type finder struct {
	bossByQuest    map[int][]int
	area3Bosses    map[int]bool // 所选区域3 Boss 的 unit_id 集合（空＝不约束）
	area5Bosses    map[int]bool
	thirdBlockType string
	perfectStart   bool
}

// targetAreas 返回需满足条件的区域（对应参考项目 _target_areas）。
func targetAreas(difficulty int) []int {
	if difficulty == 1 {
		return []int{1, 2, 3}
	}
	return []int{1, 2, 3, 4, 5}
}

// bossUnitIDs 返回某格子（Boss 关）的 unit_id 集合（对应参考项目 _boss_unit_ids）。
func (f *finder) bossUnitIDs(block lab.Block) map[int]bool {
	set := map[int]bool{}
	if block.QuestID == 0 {
		return set
	}
	for _, uid := range f.bossByQuest[block.QuestID] {
		set[uid] = true
	}
	return set
}

// bossMatches 报告某 Boss 格是否命中所选 Boss（对应参考项目 _boss_matches）。空选择＝不约束。
func (f *finder) bossMatches(area int, block lab.Block) bool {
	selected := f.selectedBosses(area)
	if len(selected) == 0 {
		return true
	}
	for uid := range f.bossUnitIDs(block) {
		if selected[uid] {
			return true
		}
	}
	return false
}

// selectedBosses 返回某区所选 Boss 集合（非 3/5 区为 nil）。
func (f *finder) selectedBosses(area int) map[int]bool {
	switch area {
	case 3:
		return f.area3Bosses
	case 5:
		return f.area5Bosses
	}
	return nil
}

// expectedBlockTypes 返回某区某列的期望格子类型集合（对应参考项目 _expected_block_types）。
func (f *finder) expectedBlockTypes(area, column int) map[int]bool {
	if (area == 3 || area == 5) && column == 3 {
		switch f.thirdBlockType {
		case "必须事件":
			return map[int]bool{5: true}
		case "两者都行":
			return map[int]bool{5: true, 6: true}
		default: // 必须遗物
			return map[int]bool{6: true}
		}
	}
	return map[int]bool{areaRequirements[area][column]: true}
}

// findRoutes 对所有目标区域逐一找满足条件的路线（对应参考项目 _find_routes）。
func (f *finder) findRoutes(difficulty int, blocks []lab.Block) (map[int][]lab.Block, string) {
	routes := map[int][]lab.Block{}
	var failures []string
	for _, area := range targetAreas(difficulty) {
		route, reason := f.findAreaRoute(area, blocks)
		if route == nil {
			failures = append(failures, reason)
		} else {
			routes[area] = route
		}
	}
	if len(failures) > 0 {
		return nil, strings.Join(failures, "；")
	}
	return routes, ""
}

// findAreaRoute 在某区域用 DFS 找一条满足条件的可达路线（对应参考项目 _find_area_route）。
func (f *finder) findAreaRoute(area int, mapList []lab.Block) ([]lab.Block, string) {
	expected := areaRequirements[area]
	var blocks []lab.Block
	for _, b := range mapList {
		if b.Area == area {
			blocks = append(blocks, b)
		}
	}
	if len(blocks) == 0 {
		return nil, fmt.Sprintf("区域%d没有地图数据", area)
	}

	byID := map[int]lab.Block{}
	byColumn := map[int][]lab.Block{}
	for _, b := range blocks {
		byID[b.BlockID] = b
		byColumn[b.Column] = append(byColumn[b.Column], b)
	}

	lastColumn := 0
	var missing []int
	for col := range expected {
		if _, ok := byColumn[col]; !ok {
			missing = append(missing, col)
		}
		if col > lastColumn {
			lastColumn = col
		}
	}
	if len(missing) > 0 {
		sort.Ints(missing)
		return nil, fmt.Sprintf("区域%d缺少列%v", area, missing)
	}

	c := &areaCtx{area: area, expected: expected, lastColumn: lastColumn, byID: byID}
	// 起点列按 row 排序，保证探索顺序确定。
	starts := append([]lab.Block(nil), byColumn[1]...)
	sort.Slice(starts, func(i, j int) bool { return starts[i].Row < starts[j].Row })
	for _, start := range starts {
		if route := f.dfs(c, start, nil, map[int]bool{start.BlockID: true}); route != nil {
			return route, ""
		}
	}
	return nil, fmt.Sprintf("区域%d没有满足条件的可达路线", area)
}

// areaCtx 是某区域 DFS 的上下文。
type areaCtx struct {
	area       int
	expected   map[int]int
	lastColumn int
	byID       map[int]lab.Block
}

// dfs 从 block 出发深搜一条满足条件的路线（对应参考项目内层 dfs）。
func (f *finder) dfs(c *areaCtx, block lab.Block, path []lab.Block, seen map[int]bool) []lab.Block {
	if f.perfectStart && !f.expectedBlockTypes(c.area, block.Column)[block.BlockType] {
		return nil
	}
	if block.Column == c.lastColumn {
		if c.expected[block.Column] == 8 && !f.bossMatches(c.area, block) {
			return nil
		}
		return appendBlock(path, block)
	}
	for _, nextID := range block.NextBlockIDList {
		if seen[nextID] {
			continue
		}
		nb, ok := c.byID[nextID]
		if !ok {
			continue
		}
		seen[nextID] = true
		if route := f.dfs(c, nb, appendBlock(path, block), seen); route != nil {
			return route
		}
		delete(seen, nextID)
	}
	return nil
}

// appendBlock 返回 path+[block] 的独立副本（避免 DFS 兄弟分支间切片别名）。
func appendBlock(path []lab.Block, block lab.Block) []lab.Block {
	out := make([]lab.Block, len(path)+1)
	copy(out, path)
	out[len(path)] = block
	return out
}

// formatRoute 把一条路线格式化为可读字符串（对应参考项目 _format_route）。
func (f *finder) formatRoute(area int, route []lab.Block, mapList []lab.Block) string {
	areaColumns := map[int][]lab.Block{}
	for _, b := range mapList {
		if b.Area == area {
			areaColumns[b.Column] = append(areaColumns[b.Column], b)
		}
	}
	var parts []string
	for _, block := range route {
		position := positionName(block, areaColumns)
		name := blockTypeName[block.BlockType]
		if name == "" {
			name = fmt.Sprintf("%d", block.BlockType)
		}
		extra := ""
		if block.BlockType == 8 {
			extra = f.bossExtra(area, block)
		}
		parts = append(parts, fmt.Sprintf("%d%s【%s%s】", block.Column, position, name, extra))
	}
	return fmt.Sprintf("区域%d：%s", area, strings.Join(parts, "-"))
}

// bossExtra 返回 Boss 格附注（命中的 Boss 名），无则空。
func (f *finder) bossExtra(area int, block lab.Block) string {
	units := f.bossUnitIDs(block)
	if len(units) == 0 {
		return ""
	}
	selected := f.selectedBosses(area)
	var candidates []int
	for uid := range units {
		if len(selected) > 0 && !selected[uid] {
			continue
		}
		if _, ok := bossNameByUnit[uid]; ok {
			candidates = append(candidates, uid)
		}
	}
	sort.Ints(candidates)
	var names []string
	for _, uid := range candidates {
		names = append(names, bossNameByUnit[uid])
	}
	if len(names) == 0 {
		return ""
	}
	return fmt.Sprintf("(%s)", strings.Join(names, "/"))
}

// positionName 把格子行位翻译为 上/中/下/合流（对应参考项目 _position_name）。
func positionName(block lab.Block, areaColumns map[int][]lab.Block) string {
	maxRow := block.Row
	for _, b := range areaColumns[block.Column] {
		if b.Row > maxRow {
			maxRow = b.Row
		}
	}
	switch maxRow {
	case 0, 1:
		return "合流"
	case 2:
		return map[int]string{1: "下", 2: "上"}[block.Row]
	case 3:
		return map[int]string{1: "下", 2: "中", 3: "上"}[block.Row]
	default:
		return fmt.Sprintf("%d", block.Row)
	}
}
