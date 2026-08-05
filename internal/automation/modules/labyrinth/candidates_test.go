package labyrinth

import (
	"context"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	lab "github.com/cca2878/go-autopcr-core/internal/client/gameapi/labyrinth"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
)

// matchedGC 造一个"迷宫已解锁 + 地图必命中"的世界，供只关心配置解析的用例复用。
func matchedGC() (*moduletest.FakeClient, *fakeLab) {
	var blocks []lab.Block
	blocks = append(blocks, linearArea(1, []int{1, 2, 4, 2, 4, 6}, 0)...)
	blocks = append(blocks, linearArea(2, []int{1, 4, 2, 6, 3, 4, 6}, 0)...)
	blocks = append(blocks, linearArea(3, []int{1, 2, 6, 4, 3, 7, 8}, 3007)...)

	fl := &fakeLab{
		top:   &lab.TopResult{EnterID: 0, ClearedDifficulties: []int{1}},
		enter: &lab.EnterResult{EnterID: 55555, Blocks: blocks},
	}
	gc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ClearedQuests: map[int]struct{}{labyrinthUnlockQuest: {}}},
		MD: fakeMDReader{lab: fakeMDLab{
			boss:   map[int][]int{3007: {312505}},
			guilds: testGuilds,
		}},
		LabyrinthAPI: fl,
	}
	return gc, fl
}

// TestCandidates_GuildsFromMasterdata 断言公会候选来自母数据：值＝guild_id、显示＝公会名、保持升序。
func TestCandidates_GuildsFromMasterdata(t *testing.T) {
	gc, _ := matchedGC()

	cands, err := startReroll{}.Candidates(context.Background(), gc)
	if err != nil {
		t.Fatalf("Candidates 出错: %v", err)
	}
	opts := cands["labyrinth_reroll_guild_id"]
	if len(opts) != len(testGuilds) {
		t.Fatalf("应有 %d 个公会候选，得 %d", len(testGuilds), len(opts))
	}
	for i, want := range testGuilds {
		if opts[i].Label != want.Name {
			t.Errorf("第 %d 个候选显示名应为 %q，得 %q", i, want.Name, opts[i].Label)
		}
	}
	// 值必须是 guild_id 而非公会名——它要原样送进 labyrinth/enter。
	if opts[1].Value != "5" {
		t.Errorf("guild_id=5 的候选值应为 \"5\"，得 %q", opts[1].Value)
	}
}

// TestCandidates_NoMasterdata 断言母数据缺席时响亮失败（而非静默退回零校验）。
func TestCandidates_NoMasterdata(t *testing.T) {
	gc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ClearedQuests: map[int]struct{}{labyrinthUnlockQuest: {}}},
	}
	if _, err := (startReroll{}).Candidates(context.Background(), gc); err == nil {
		t.Fatal("母数据未启用时 Candidates 应报错")
	}
}

// TestStartReroll_RejectsGuildNotInWorld 是本次接线的核心收益：母数据里没有的公会会被'校验挡下'，
// 而不是照原样发给服务器。接 Candidates 之前该参数是裸 int，填 99 会一路发包。
func TestStartReroll_RejectsGuildNotInWorld(t *testing.T) {
	gc, fl := matchedGC()

	res := moduletest.RunOne(gc, startReroll{}, map[string]any{
		"labyrinth_reroll_difficulty": "1",
		"labyrinth_reroll_guild_id":   "99",
	})

	if res.Status != automation.StatusError {
		t.Fatalf("未知公会应被挡下(StatusError)，得 %s", res.Status)
	}
	// 必须是'候选集'拒绝而非类型错误——否则本用例会因"字符串 vs int"这种无关理由假绿。
	msg := res.Err.Error()
	if !strings.Contains(msg, "labyrinth_reroll_guild_id") || !strings.Contains(msg, "[4 5 6]") {
		t.Errorf("错误应点名该参数并列出世界给出的候选，得 %v", res.Err)
	}
	if fl.enterGuildID != 0 {
		t.Errorf("校验失败时不应发出 Enter，却收到 guild_id=%d", fl.enterGuildID)
	}
}

// TestStartReroll_AcceptsWorldGuild 断言世界里存在的公会通过校验，且其 guild_id 原样送达发包处
// （覆盖 ParamChoice 的字符串值 → int 的还原）。
func TestStartReroll_AcceptsWorldGuild(t *testing.T) {
	gc, fl := matchedGC()

	res := moduletest.RunOne(gc, startReroll{}, map[string]any{
		"labyrinth_reroll_difficulty": "1",
		"labyrinth_reroll_guild_id":   "6",
	})

	if res.Status != automation.StatusOK {
		t.Fatalf("状态应 OK，得 %s（%v）", res.Status, res.Err)
	}
	if fl.enterGuildID != 6 {
		t.Errorf("应以 guild_id=6 进入，得 %d", fl.enterGuildID)
	}
}

// TestStartReroll_DefaultGuild 断言不填公会时走默认 5（对应参考项目 LabyrinthGuildConfig 的 default）。
func TestStartReroll_DefaultGuild(t *testing.T) {
	gc, fl := matchedGC()

	res := moduletest.RunOne(gc, startReroll{}, map[string]any{
		"labyrinth_reroll_difficulty": "1",
	})

	if res.Status != automation.StatusOK {
		t.Fatalf("状态应 OK，得 %s（%v）", res.Status, res.Err)
	}
	if fl.enterGuildID != 5 {
		t.Errorf("默认应以 guild_id=5 进入，得 %d", fl.enterGuildID)
	}
}
