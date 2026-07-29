package labyrinth

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// startRerollV2 是「黎明界刷开局」的重新设计版，与 startReroll【并存】以便实测对比。
//
// 与 v1 的唯一实质差别在【判定】：v1 逐列比对固定模板（某列必须是某类格子），v2 求路线取到的
// 贵重格数量、与这张图自身的上界比较。位置无关，因而不会重蹈 v1 那处结构性错配——模板要求
// EX 怪同时在第3、5列，可生成器有一半的图把第二个 EX 放在第6列，那些图无论怎么走都判死。
//
// 「要什么」由两组参数表达，都不提列号：
//   - 允许少拿几格：0＝把能拿的都拿到；放宽一格通常能把重开次数降一个量级。
//   - 三对「同列二选一」偏好：角色⇄遗物、遗物⇄商店、遗物⇄事件。它们只在同列相遇时才需要
//     取舍，所以按【类型对】而非列号表达，生成器挪位置也不会失效。
//
// core 不知道任何分布数据：上界由地图自身算出。「这组条件要刷多少次」是外壳/文档的事。
type startRerollV2 struct{}

// 参数名前缀。与 v1 完全分开，两个模块可同时出现在配置里互不干扰。
const (
	v2Difficulty    = "labyrinth_reroll_v2_difficulty"
	v2GuildID       = "labyrinth_reroll_v2_guild_id"
	v2Allowance     = "labyrinth_reroll_v2_allowance"
	v2MaxCount      = "labyrinth_reroll_v2_max_count"
	v2CharVsRelic   = "labyrinth_reroll_v2_char_vs_relic"
	v2RelicVsShop   = "labyrinth_reroll_v2_relic_vs_shop"
	v2Area3Third    = "labyrinth_reroll_v2_area3_relic_vs_event"
	v2Area5Third    = "labyrinth_reroll_v2_area5_relic_vs_event"
	v2Area3Boss     = "labyrinth_reroll_v2_area3_boss"
	v2Area5Boss     = "labyrinth_reroll_v2_area5_boss"
	v2PreferEither  = "都行"
	v2ObservedKind  = "labyrinth_map" // 与 v1 同一 kind：地图样本本身同质，合起来才够分析
	v2ModuleTagName = "module"
)

func (startRerollV2) Meta() automation.Meta {
	return automation.Meta{
		Name:  "labyrinth_start_reroll_v2",
		Title: "黎明界刷开局 v2（新版·实验中）",
		Description: "刷开局的重新设计版，与原模块并存以便对比。判定改为「路线取到的贵重格数量」" +
			"而非逐列比对固定模板：位置无关，不会因生成器把某类格子挪到别的列而误判。" +
			"可指定允许少拿几格与三对同列取舍。实验中，结果请与原模块对照。",
		Category:        "黎明界",
		NeedsMasterdata: true,
	}
}

func (startRerollV2) Params() []automation.Param {
	return []automation.Param{
		{Name: v2Difficulty, Type: automation.ParamChoice, Default: "5",
			Description: "难度", Bounds: automation.Bounds{Choices: []string{"1", "2", "3", "4", "5"}}},
		// 公会候选依赖母数据，由 Candidates 在世界已知时填（同 v1）。
		{Name: v2GuildID, Type: automation.ParamChoice, Default: "5", Description: "公会"},
		// 【逐区域】生效：每个目标区域各自最多少拿这么多格，不是五区合计。
		{Name: v2Allowance, Type: automation.ParamChoice, Default: "0",
			Description: "每个区域允许少拿几格（0=完美）",
			Bounds:      automation.Bounds{Choices: []string{"0", "1", "2", "3"}}},
		{Name: v2MaxCount, Type: automation.ParamChoice, Default: "1000",
			Description: "最多重开次数", Bounds: automation.Bounds{Choices: []string{"100", "1000", "2000"}}},

		{Name: v2CharVsRelic, Type: automation.ParamChoice, Default: "角色",
			Description: "同格二选一：角色 or 遗物",
			Bounds:      automation.Bounds{Choices: []string{"角色", "遗物", v2PreferEither}}},
		{Name: v2RelicVsShop, Type: automation.ParamChoice, Default: "遗物",
			Description: "同格二选一：遗物 or 商店",
			Bounds:      automation.Bounds{Choices: []string{"遗物", "商店", v2PreferEither}}},
		// 区域3 与区域5 分开：同一个选择在两区代价不同，共用一个值会让人在一区付了代价、
		// 另一区却一无所得。
		{Name: v2Area3Third, Type: automation.ParamChoice, Default: v2PreferEither,
			Description: "区域3 同格二选一：遗物 or 事件",
			Bounds:      automation.Bounds{Choices: []string{"遗物", "事件", v2PreferEither}}},
		{Name: v2Area5Third, Type: automation.ParamChoice, Default: v2PreferEither,
			Description: "区域5 同格二选一：遗物 or 事件",
			Bounds:      automation.Bounds{Choices: []string{"遗物", "事件", v2PreferEither}}},

		{Name: v2Area3Boss, Type: automation.ParamMultiChoice, Default: simpleBossNamesOf(area3Bosses),
			Description: "区域3 Boss", Bounds: automation.Bounds{Choices: bossNamesOf(area3Bosses)}},
		{Name: v2Area5Boss, Type: automation.ParamMultiChoice, Default: simpleBossNamesOf(area5Bosses),
			Description: "区域5 Boss", Bounds: automation.Bounds{Choices: bossNamesOf(area5Bosses)}},
	}
}

// Candidates 同 v1：公会来自母数据。
func (startRerollV2) Candidates(ctx context.Context, gc client.GameClient) (map[string][]automation.Option, error) {
	md := gc.Masterdata()
	if md == nil {
		return nil, automation.RequireMasterdata("解析黎明界公会候选")
	}
	guilds, err := md.Labyrinth().EnterGuilds(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]automation.Option, len(guilds))
	for i, g := range guilds {
		opts[i] = automation.Option{Value: strconv.Itoa(g.ID), Label: g.Name}
	}
	return map[string][]automation.Option{v2GuildID: opts}, nil
}

func (startRerollV2) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	if !gc.Data().IsQuestCleared(labyrinthUnlockQuest) {
		return automation.Skip("迷宫未解锁")
	}
	md := gc.Masterdata()
	if md == nil {
		return automation.RequireMasterdata("刷黎明界开局")
	}
	bossByQuest, err := md.Labyrinth().BossUnitIDsByQuest(ctx)
	if err != nil {
		return err
	}

	difficulty := choiceInt(rc.String(v2Difficulty), 5)
	guildID := choiceInt(rc.String(v2GuildID), 5)
	allowance := choiceInt(rc.String(v2Allowance), 0)
	maxCount := choiceInt(rc.String(v2MaxCount), 1000)

	v := &valuer{
		bossByQuest: bossByQuest,
		area3Bosses: unitSetOf(rc.Strings(v2Area3Boss), area3Bosses),
		area5Bosses: unitSetOf(rc.Strings(v2Area5Boss), area5Bosses),
		prefs:       prefsFrom(rc),
	}

	top, err := gc.Labyrinth().Top(ctx)
	if err != nil {
		return err
	}
	maxUnlocked := maxUnlockedDifficulty(top)
	if difficulty > maxUnlocked {
		rc.Logf("黎明界难度%d尚未解锁，当前最大可挑战难度为%d，跳过执行。", difficulty, maxUnlocked)
		return fmt.Errorf("未解锁所选黎明界难度")
	}
	if top.EnterID != 0 {
		rc.Logf("检测到已有黎明界开局，先撤退。")
		if err := gc.Labyrinth().Retire(ctx, top.EnterID); err != nil {
			return err
		}
	}

	lastReason := ""
	for attempt := 1; attempt <= maxCount; attempt++ {
		enter, err := gc.Labyrinth().Enter(ctx, guildID, difficulty)
		if err != nil {
			return err
		}
		routes, worst, reason := v.judgeAll(difficulty, enter.Blocks, allowance)
		emitMapV2(rc, v, difficulty, guildID, attempt, maxCount, allowance, enter.Blocks, routes != nil)

		if routes != nil {
			rc.Logf("刷到目标开局，总尝试次数：%d（最差区域少拿 %d 格，允许 %d）", attempt, worst, allowance)
			for _, area := range targetAreas(difficulty) {
				if r, ok := routes[area]; ok {
					rc.Logf("%s", formatValueRoute(v, area, r, enter.Blocks))
				}
			}
			return nil
		}

		lastReason = reason
		if enter.EnterID != 0 {
			if err := gc.Labyrinth().Retire(ctx, enter.EnterID); err != nil {
				return err
			}
		}
		if _, err := gc.Labyrinth().Top(ctx); err != nil {
			return err
		}
	}
	return fmt.Errorf("重开%d次仍未刷到目标开局，最后失败原因：%s", maxCount, lastReason)
}

// judgeAll 逐区判定。全部达标才算命中；否则给出最短板的说明。
// worst 是达标时各区里最大的「少拿格数」，供日志说明这次到底拿到了什么。
func (v *valuer) judgeAll(difficulty int, blocks []lab.Block, allowance int) (map[int][]lab.Block, int, string) {
	routes := map[int][]lab.Block{}
	var failures []string
	worst := 0
	for _, area := range targetAreas(difficulty) {
		route, short, ok := v.judge(area, blocks, allowance)
		switch {
		case ok:
			routes[area] = route
			if short > worst {
				worst = short
			}
		case short == unreachable:
			failures = append(failures, fmt.Sprintf("区域%d没有满足Boss条件的可达路线", area))
		default:
			failures = append(failures, fmt.Sprintf("区域%d少拿%d格（上限%d）", area, short, allowance))
		}
	}
	if len(failures) > 0 {
		return nil, 0, strings.Join(failures, "；")
	}
	return routes, worst, ""
}

// prefsFrom 把三个「同列二选一」参数翻成判定用的偏好。
func prefsFrom(rc *automation.RunContext) []pref {
	byName := map[string]int{"角色": blockChar, "遗物": blockRelic, "商店": blockShop, "事件": blockEvent}
	pick := func(param string, a, b int) int {
		switch v := rc.String(param); v {
		case v2PreferEither, "":
			return 0
		default:
			if t, ok := byName[v]; ok && (t == a || t == b) {
				return t
			}
			return 0
		}
	}
	return []pref{
		{a: blockChar, b: blockRelic, winner: pick(v2CharVsRelic, blockChar, blockRelic)},
		{a: blockRelic, b: blockShop, winner: pick(v2RelicVsShop, blockRelic, blockShop)},
		{area: 3, a: blockRelic, b: blockEvent, winner: pick(v2Area3Third, blockRelic, blockEvent)},
		{area: 5, a: blockRelic, b: blockEvent, winner: pick(v2Area5Third, blockRelic, blockEvent)},
	}
}

// emitMapV2 发射地图样本。
//
// 沿用 v1 的 kind：地图本身是同质样本，两个模块采到的合起来才够估生成分布。载荷里带
// `module` 标出来源与本模块特有的判定参数——**没有该字段的记录即 v1**（v1 有意保持不动）。
func emitMapV2(rc *automation.RunContext, v *valuer, difficulty, guildID, attempt, maxCount, allowance int,
	blocks []lab.Block, matched bool,
) {
	arr := make([]map[string]any, len(blocks))
	for i, b := range blocks {
		var bossUnits []int
		if b.BlockType == blockBoss {
			bossUnits = append(bossUnits, v.bossByQuest[b.QuestID]...)
			sortInts(bossUnits)
		}
		arr[i] = map[string]any{
			"area": b.Area, "column": b.Column, "row": b.Row,
			"block_type": b.BlockType, "block_id": b.BlockID, "quest_id": b.QuestID,
			"next": b.NextBlockIDList, "boss_units": bossUnits,
		}
	}
	// 少拿格数逐区上报。Boss 不命中的区域【整个键缺席】，不记 0——那不是「少拿 0 格」而是
	// 「没有可达路线」，两者混同会让分析侧把最差的样本当成最好的。缺席的区域仍可从 blocks
	// 与 areaN_boss 重算结构性少拿数，信息没丢。
	shortfall := map[string]int{}
	for _, area := range targetAreas(difficulty) {
		if _, short, _ := v.judge(area, blocks, allowance); short >= 0 {
			shortfall[strconv.Itoa(area)] = short
		}
	}
	rc.Emit(v2ObservedKind, map[string]any{
		v2ModuleTagName: "labyrinth_start_reroll_v2",
		"difficulty":    difficulty,
		"guild_id":      guildID,
		"matched":       matched,
		"blocks":        arr,
		// 停止规则与判定条件，供分析侧检验前提、切会话、跨账号比对。
		"attempt":        attempt,
		"max_count":      maxCount,
		"allowance":      allowance,
		"char_vs_relic":  rc.String(v2CharVsRelic),
		"relic_vs_shop":  rc.String(v2RelicVsShop),
		"area3_third":    rc.String(v2Area3Third),
		"area5_third":    rc.String(v2Area5Third),
		"area3_boss":     sortedKeys(v.area3Bosses),
		"area5_boss":     sortedKeys(v.area5Bosses),
		"area_shortfall": shortfall,
	})
}

// formatValueRoute 把一条路线格式化为可读字符串，并标出哪几格是本次判定认可的贵重格。
func formatValueRoute(v *valuer, area int, route []lab.Block, mapList []lab.Block) string {
	areaColumns := map[int][]lab.Block{}
	for _, b := range mapList {
		if b.Area == area {
			areaColumns[b.Column] = append(areaColumns[b.Column], b)
		}
	}
	valuable := v.columnValuable(area, mapList)
	got := 0
	var parts []string
	for _, block := range route {
		name := blockTypeName[block.BlockType]
		if name == "" {
			name = strconv.Itoa(block.BlockType)
		}
		mark := ""
		if valuable[block.Column][block.BlockType] {
			mark, got = "*", got+1
		}
		parts = append(parts, fmt.Sprintf("%d%s【%s%s】", block.Column, positionName(block, areaColumns), name, mark))
	}
	return fmt.Sprintf("区域%d（贵重 %d/%d，*为计入）：%s", area, got, v.bound(area, mapList), strings.Join(parts, "-"))
}

// sortInts 就地升序（unit_id 列表短，插入排序足够，且不必引入依赖）。
func sortInts(xs []int) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
