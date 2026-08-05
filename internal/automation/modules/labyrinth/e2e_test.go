package labyrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdlab "github.com/cca2878/go-autopcr-core/internal/client/masterdata/labyrinth"
)

// --- 假黎明界能力面（gameapi） ---

type fakeLab struct {
	top          *lab.TopResult
	enter        *lab.EnterResult
	retired      []int
	enterGuildID int // 最近一次 Enter 收到的 guild_id（验证配置真的流到了发包处）
}

func (f *fakeLab) Top(context.Context) (*lab.TopResult, error) { return f.top, nil }
func (f *fakeLab) Enter(_ context.Context, guildID, _ int) (*lab.EnterResult, error) {
	f.enterGuildID = guildID
	return f.enter, nil
}
func (f *fakeLab) Retire(_ context.Context, id int) error {
	f.retired = append(f.retired, id)
	return nil
}

// --- 假黎明界母数据 ---

// testGuilds 仿母数据 labyrinth_enter_guild：guild_id 升序，名字已抹平换行标记。
var testGuilds = []mdlab.Guild{
	{ID: 4, Name: "破晓之星"},
	{ID: 5, Name: "美食殿堂"},
	{ID: 6, Name: "小小甜心"},
}

type fakeMDLab struct {
	boss   map[int][]int
	guilds []mdlab.Guild
}

func (f fakeMDLab) BossUnitIDsByQuest(context.Context) (map[int][]int, error) { return f.boss, nil }
func (f fakeMDLab) EnterGuilds(context.Context) ([]mdlab.Guild, error)        { return f.guilds, nil }

type fakeMDReader struct {
	masterdata.Reader
	lab mdlab.API
}

func (f fakeMDReader) Labyrinth() mdlab.API { return f.lab }

// linearArea 造某区域一条单链（col 1..len(types)，各 row1，id＝area*100+col），末列带 quest_id。
func linearArea(area int, types []int, lastQuestID int) []lab.Block {
	n := len(types)
	bs := make([]lab.Block, n)
	for i := range n {
		col := i + 1
		id := area*100 + col
		var next []int
		if col < n {
			next = []int{area*100 + col + 1}
		}
		q := 0
		if col == n {
			q = lastQuestID
		}
		bs[i] = lab.Block{Area: area, Column: col, Row: 1, BlockID: id, BlockType: types[i], QuestID: q, NextBlockIDList: next}
	}
	return bs
}

// TestStartReroll_E2E 端到端跑"黎明界刷开局"：合成一张难度1(区域1-3)的完美地图 + 区域3 Boss，
// 完美开局一次命中；接真实 Collector 抓遥测，打印详细结果与 labyrinth_map 观测。
func TestStartReroll_E2E(t *testing.T) {
	// 完美模板：area1[1,2,4,2,4,6]、area2[1,4,2,6,3,4,6]、area3[1,2,6,4,3,7,8]（末列 col7＝Boss）。
	var blocks []lab.Block
	blocks = append(blocks, linearArea(1, []int{1, 2, 4, 2, 4, 6}, 0)...)
	blocks = append(blocks, linearArea(2, []int{1, 4, 2, 6, 3, 4, 6}, 0)...)
	blocks = append(blocks, linearArea(3, []int{1, 2, 6, 4, 3, 7, 8}, 3007)...) // 区域3 Boss 关 quest_id=3007

	fl := &fakeLab{
		top:   &lab.TopResult{EnterID: 0, ClearedDifficulties: []int{1}}, // 已通关难度1 → 解锁到2
		enter: &lab.EnterResult{EnterID: 55555, Blocks: blocks},
	}
	gc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ClearedQuests: map[int]struct{}{labyrinthUnlockQuest: {}}}, // 迷宫已解锁
		MD: fakeMDReader{lab: fakeMDLab{
			boss:   map[int][]int{3007: {312505}}, // 3007→厄勒克特拉夫人(简单，默认已选)
			guilds: testGuilds,
		}},
		LabyrinthAPI: fl,
	}

	// 真实 Collector：捕获模块 emit 的观测。
	var obs []automation.Observation
	col := func(o automation.Observation) { obs = append(obs, o) }

	reg := automation.NewRegistry()
	reg.Register(startReroll{})
	tasks := []automation.Task{{Module: "labyrinth_start_reroll", Values: map[string]any{
		"labyrinth_reroll_difficulty":    "1",
		"labyrinth_reroll_perfect_start": true,
	}}}

	results, err := automation.Run(context.Background(), gc, reg, tasks, nil, col)
	if err != nil {
		t.Fatalf("Run 出错: %v", err)
	}
	res := results[0]

	// --- 打印详细结果 ---
	fmt.Println("========== 黎明界刷开局 · 运行结果 ==========")
	fmt.Printf("状态: %s\n", res.Status)
	fmt.Println("过程日志:")
	for _, line := range res.Log {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("撤退调用: enter_id=%v\n", fl.retired)

	// --- 打印遥测 ---
	fmt.Println("\n========== 遥测发射 (Collector 捕获) ==========")
	fmt.Printf("共 %d 条观测\n", len(obs))
	for _, o := range obs {
		js, _ := json.MarshalIndent(o.Fields, "  ", "  ")
		fmt.Printf("Kind=%s\n  %s\n", o.Kind, js)
	}
	fmt.Println("============================================")

	// --- 断言（保证这是真跑而非空壳） ---
	if res.Status != automation.StatusOK {
		t.Fatalf("状态应 OK，得 %s（%v）", res.Status, res.Err)
	}
	if len(obs) != 1 || obs[0].Kind != "labyrinth_map" {
		t.Fatalf("应发射 1 条 labyrinth_map，得 %d 条", len(obs))
	}
	if m, _ := obs[0].Fields["matched"].(bool); !m {
		t.Fatal("本次地图应命中(matched=true)")
	}
}
