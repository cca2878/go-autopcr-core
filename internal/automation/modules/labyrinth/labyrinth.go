// Package labyrinth 汇集「黎明界（迷宫）」域的自动化模块（刷开局；对应 ref labyrint.py）。
//
// 本文件是模块【编排】：解锁判定、配置解析、进入/撤退重刷循环与地图遥测发射；纯路线判定算法
// 独立在 route.go（可独立单测）。
package labyrinth

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
)

// labyrinthUnlockQuest 是「黎明界」的解锁任务（对应 ref labyrinth_top 的 is_quest_cleared(11065001)）。
const labyrinthUnlockQuest = 11065001

// bossInfo 是一个 Boss 的（unit_id, 名称, 难度）。
type bossInfo struct {
	unitID     int
	name       string
	difficulty string
}

var area3Bosses = []bossInfo{
	{312505, "厄勒克特拉夫人", "简单"},
	{319604, "冰霜魔狼", "简单"},
	{303306, "暗黑滴水嘴兽", "普通"},
	{301206, "巨型魔像", "普通"},
	{306604, "毒液沙鳗蛇", "困难"},
}

var area5Bosses = []bossInfo{
	{310103, "愤怒巨龙", "简单"},
	{301701, "炸脖龙", "普通"},
	{319401, "究极守护者", "普通"},
	{315004, "领主哥布林", "困难"},
	{302501, "奇美拉", "困难"},
}

// bossNamesOf 返回某区 Boss 名列表（供配置候选）。
func bossNamesOf(bosses []bossInfo) []string {
	out := make([]string, len(bosses))
	for i, b := range bosses {
		out[i] = b.name
	}
	return out
}

// simpleBossNamesOf 返回某区「简单」Boss 名列表（供配置默认）。
func simpleBossNamesOf(bosses []bossInfo) []string {
	var out []string
	for _, b := range bosses {
		if b.difficulty == "简单" {
			out = append(out, b.name)
		}
	}
	return out
}

// unitSetOf 把 Boss 名列表转为 unit_id 集合（以该区名单反查）。
func unitSetOf(names []string, bosses []bossInfo) map[int]bool {
	nameToUnit := map[string]int{}
	for _, b := range bosses {
		nameToUnit[b.name] = b.unitID
	}
	set := map[int]bool{}
	for _, n := range names {
		if uid, ok := nameToUnit[n]; ok {
			set[uid] = true
		}
	}
	return set
}

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(startReroll{})
	r.Register(startRerollV2{}) // 重新设计版，与 v1 并存以便实测对比
}

// startReroll 是「黎明界刷开局」模块（对应 ref labyrinth_start_reroll）。
//
// 反复「进入→判定路线→撤退重刷」直至刷到满足条件的地图：完美开局要求路线逐列吻合模板（不错过 EX 关
// 与必要遗物），并可指定区域 3/5 的 Boss 与第 3 格类型。每次进入的地图经 rc.Emit 发射（生成分布采样）。
type startReroll struct{}

func (startReroll) Meta() automation.Meta {
	return automation.Meta{
		Name:            "labyrinth_start_reroll",
		Title:           "黎明界刷开局",
		Description:     "反复进入黎明界直至刷到满足条件的开局（可选完美开局/指定Boss/第3格）；每次进入的地图经遥测发射。",
		Category:        "黎明界",
		NeedsMasterdata: true,
	}
}

func (startReroll) Params() []automation.Param {
	return []automation.Param{
		{Name: "labyrinth_reroll_difficulty", Type: automation.ParamChoice, Default: "5",
			Description: "难度", Bounds: automation.Bounds{Choices: []string{"1", "2", "3", "4", "5"}}},
		// 公会候选依赖母数据，故无静态 Choices——由 Candidates 在世界已知时填。
		{Name: "labyrinth_reroll_guild_id", Type: automation.ParamChoice, Default: "5",
			Description: "公会"},
		{Name: "labyrinth_reroll_perfect_start", Type: automation.ParamBool, Default: false, Description: "完美开局"},
		{Name: "labyrinth_reroll_max_count", Type: automation.ParamChoice, Default: "100",
			Description: "最多重开次数（完美开局）", Bounds: automation.Bounds{Choices: []string{"100", "1000", "2000"}}},
		{Name: "labyrinth_reroll_third_block_type", Type: automation.ParamChoice, Default: "两者都行",
			Description: "区域3/5第3格", Bounds: automation.Bounds{Choices: []string{"必须遗物", "必须事件", "两者都行"}}},
		{Name: "labyrinth_reroll_area3_boss", Type: automation.ParamMultiChoice, Default: simpleBossNamesOf(area3Bosses),
			Description: "区域3Boss", Bounds: automation.Bounds{Choices: bossNamesOf(area3Bosses)}},
		{Name: "labyrinth_reroll_area5_boss", Type: automation.ParamMultiChoice, Default: simpleBossNamesOf(area5Bosses),
			Description: "区域5Boss", Bounds: automation.Bounds{Choices: bossNamesOf(area5Bosses)}},
	}
}

// Candidates 把「公会」解析成母数据里可进入的公会：值是 guild_id、显示是公会名（对应 ref
// LabyrinthGuildConfig——candidates=db.labyrinth_enter_guild、candidate_display=guild_name）。
func (startReroll) Candidates(ctx context.Context, gc client.GameClient) (map[string][]automation.Option, error) {
	md := gc.Masterdata()
	if md == nil {
		return nil, fmt.Errorf("黎明界刷开局需要母数据，但未启用")
	}
	guilds, err := md.Labyrinth().EnterGuilds(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]automation.Option, len(guilds))
	for i, g := range guilds {
		opts[i] = automation.Option{Value: strconv.Itoa(g.ID), Label: g.Name}
	}
	return map[string][]automation.Option{"labyrinth_reroll_guild_id": opts}, nil
}

func (startReroll) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	if !gc.Data().IsQuestCleared(labyrinthUnlockQuest) {
		return automation.Skip("迷宫未解锁")
	}
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("黎明界刷开局需要母数据，但未启用")
	}
	bossByQuest, err := md.Labyrinth().BossUnitIDsByQuest(ctx)
	if err != nil {
		return err
	}

	difficulty := choiceInt(rc.String("labyrinth_reroll_difficulty"), 5)
	guildID := choiceInt(rc.String("labyrinth_reroll_guild_id"), 5)
	perfectStart := rc.Bool("labyrinth_reroll_perfect_start")
	maxCount := 100
	if perfectStart {
		maxCount = choiceInt(rc.String("labyrinth_reroll_max_count"), 100)
	}

	f := &finder{
		bossByQuest:    bossByQuest,
		area3Bosses:    unitSetOf(rc.Strings("labyrinth_reroll_area3_boss"), area3Bosses),
		area5Bosses:    unitSetOf(rc.Strings("labyrinth_reroll_area5_boss"), area5Bosses),
		thirdBlockType: rc.String("labyrinth_reroll_third_block_type"),
		perfectStart:   perfectStart,
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
		routes, reason := f.findRoutes(difficulty, enter.Blocks)
		f.emitMap(rc, difficulty, guildID, attempt, maxCount, enter.Blocks, routes != nil)

		if routes != nil {
			perfect := ""
			if perfectStart {
				perfect = "完美"
			}
			rc.Logf("刷到%s路线，总尝试次数：%d", perfect, attempt)
			for _, area := range slices.Sorted(maps.Keys(routes)) {
				rc.Logf("%s", f.formatRoute(area, routes[area], enter.Blocks))
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

	return fmt.Errorf("重开%d次仍未刷到目标路线，最后失败原因：%s", maxCount, lastReason)
}

// maxUnlockedDifficulty 返回当前最大可挑战难度（对应 ref _max_unlocked_difficulty）。
func maxUnlockedDifficulty(top *lab.TopResult) int {
	if len(top.ClearedDifficulties) == 0 {
		return 1
	}
	return min(slices.Max(top.ClearedDifficulties)+1, 5)
}

// emitMap 发射一次进入的地图（生成分布采样：每格 area/column/row/type/quest/boss + 是否命中）。
//
// 除地图本身还带上**停止规则**（attempt/max_count + 匹配条件）。循环命中即停，
// 但「首次命中」是个停止时间——{N≥i} 只由前 i-1 次抽取决定——故由 Wald 恒等式，
// 池化频次对地图生成分布仍然一致，重掷样本不作废。attempt 的价值不在去偏，而在让
// 分析侧能**检验**这个前提（命中是否真的只出现在末次）、能按会话聚类算标准误
// （独立单元是会话而非格子），并能识别跑满 max_count 而截断的会话。匹配条件因人而异，
// 不记录则 matched 在账号间不可比。会话边界由同一批次内 attempt 的重复值界定，
// 无需在 core 里造随机 run id。
func (f *finder) emitMap(rc *automation.RunContext, difficulty, guildID, attempt, maxCount int, blocks []lab.Block, matched bool) {
	arr := make([]map[string]any, len(blocks))
	for i, b := range blocks {
		var bossUnits []int
		if b.BlockType == 8 {
			for uid := range f.bossUnitIDs(b) {
				bossUnits = append(bossUnits, uid)
			}
			slices.Sort(bossUnits)
		}
		arr[i] = map[string]any{
			"area": b.Area, "column": b.Column, "row": b.Row,
			"block_type": b.BlockType, "block_id": b.BlockID, "quest_id": b.QuestID,
			"next": b.NextBlockIDList, "boss_units": bossUnits,
		}
	}
	rc.Emit("labyrinth_map", map[string]any{
		"difficulty": difficulty,
		"guild_id":   guildID,
		"matched":    matched,
		"blocks":     arr,
		// 停止规则与匹配条件，供分析侧检验前提、切会话、跨账号比对。
		"attempt":          attempt,
		"max_count":        maxCount,
		"perfect_start":    f.perfectStart,
		"third_block_type": f.thirdBlockType,
		"area3_boss":       sortedKeys(f.area3Bosses),
		"area5_boss":       sortedKeys(f.area5Bosses),
	})
}

// sortedKeys 把 unit_id 集合摊成有序切片，让遥测载荷对同一组选择稳定可比。
func sortedKeys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// choiceInt 把单选字符串转为 int（失败取 def）。
func choiceInt(s string, def int) int {
	n := 0
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}
