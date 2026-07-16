package exequip

import (
	"context"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	apiexequip "github.com/cca2878/go-autopcr-core/internal/client/gameapi/exequip"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdexequip "github.com/cca2878/go-autopcr-core/internal/client/masterdata/exequip"
)

// --- 假能力面 ---

type fakeAlces struct {
	top      *apiexequip.AlcesPending
	fixed    []int
	canceled []int
	locks    []lockCall
}

type lockCall struct {
	serial, slot int
	lock         bool
}

func (f *fakeAlces) AlcesTop(context.Context) (*apiexequip.AlcesPending, error) { return f.top, nil }
func (f *fakeAlces) AlcesExec(context.Context, int, int, int) (*apiexequip.AlcesPending, error) {
	return nil, nil
}
func (f *fakeAlces) AlcesFixResult(_ context.Context, serial int) error {
	f.fixed = append(f.fixed, serial)
	return nil
}
func (f *fakeAlces) AlcesCancelResult(_ context.Context, serial int) error {
	f.canceled = append(f.canceled, serial)
	return nil
}
func (f *fakeAlces) AlcesLockSlot(_ context.Context, serial, slot int, lock bool) error {
	f.locks = append(f.locks, lockCall{serial, slot, lock})
	return nil
}

type fakeMDExequip struct {
	snap *mdexequip.Snapshot
}

func (fakeMDExequip) RarityByID(context.Context) (map[int]int, error) { return nil, nil }
func (fakeMDExequip) AlcesCost(context.Context) (map[mdexequip.ItemKey]int, error) {
	return map[mdexequip.ItemKey]int{}, nil
}
func (f fakeMDExequip) LoadSnapshot(context.Context) (*mdexequip.Snapshot, error) { return f.snap, nil }

type fakeReader struct {
	masterdata.Reader
	ex mdexequip.API
}

func (f fakeReader) Exequip() mdexequip.API { return f.ex }

// --- computeWeight：字典序编码 ---

func TestComputeWeight_Lexicographic(t *testing.T) {
	// 目标 物攻(2)；优先级 物贯(12) > 法贯(13) > 物攻(2,目标) > 魔攻(4)。
	w := computeWeight(map[int]int{2: 1}, []int{12, 13, 2, 4})

	if w[2] != 27000 { // 目标属性取最终 base（最高）
		t.Fatalf("目标 物攻 权重应为最高 27000，得 %d", w[2])
	}
	if w[12] <= w[13] || w[13] <= w[4] {
		t.Fatalf("非目标属性应按优先级递减：物贯%d 法贯%d 魔攻%d", w[12], w[13], w[4])
	}
	if w[13] != 30*w[4] || w[12] != 30*w[13] {
		t.Fatalf("相邻优先级应为 30 倍：魔攻%d 法贯%d 物贯%d", w[4], w[13], w[12])
	}
	// 字典序性质：step 上限 5 一档也压不过高一级属性。
	if 5*w[13] >= w[12] {
		t.Fatalf("5×法贯(%d) 不应 ≥ 物贯(%d)", 5*w[13], w[12])
	}
}

// --- decideAlces：采纳/放弃 ---

func newRun(gc *moduletest.FakeClient, target map[int]int, weight map[int]int) *enhanceRun {
	return &enhanceRun{gc: gc, rc: &automation.RunContext{}, serialID: 100, exID: 1, target: target, weight: weight}
}

func gcWith(sub []gamestate.ExEquipSubStatus, fx *fakeAlces) *moduletest.FakeClient {
	return &moduletest.FakeClient{
		State: &gamestate.PlayerState{ExEquips: map[int]gamestate.ExEquip{
			100: {SerialID: 100, ExEquipmentID: 1, SubStatus: sub},
		}},
		ExequipAPI: fx,
	}
}

func TestDecideAlces_AcceptOnNewMaxTarget(t *testing.T) {
	fx := &fakeAlces{}
	gc := gcWith([]gamestate.ExEquipSubStatus{{Status: 2, Step: 3}}, fx) // 当前 物攻 step3
	r := newRun(gc, map[int]int{2: 1}, computeWeight(map[int]int{2: 1}, []int{2}))

	pending := &apiexequip.AlcesPending{SerialID: 100, SubStatus: []apiexequip.SubStatus{{SlotNumber: 1, Status: 2, Step: 5}}}
	accept, err := r.decideAlces(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if !accept || len(fx.fixed) != 1 || len(fx.canceled) != 0 {
		t.Fatalf("命中新满级目标应采纳(fix)，得 accept=%v fixed=%v canceled=%v", accept, fx.fixed, fx.canceled)
	}
}

func TestDecideAlces_RejectWhenWorse(t *testing.T) {
	fx := &fakeAlces{}
	gc := gcWith([]gamestate.ExEquipSubStatus{{Status: 2, Step: 5}}, fx) // 当前已满级 物攻
	r := newRun(gc, map[int]int{2: 1}, computeWeight(map[int]int{2: 1}, []int{2}))

	pending := &apiexequip.AlcesPending{SerialID: 100, SubStatus: []apiexequip.SubStatus{{SlotNumber: 1, Status: 2, Step: 2}}}
	accept, err := r.decideAlces(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if accept || len(fx.canceled) != 1 || len(fx.fixed) != 0 {
		t.Fatalf("更差应放弃(cancel)，得 accept=%v fixed=%v canceled=%v", accept, fx.fixed, fx.canceled)
	}
}

func TestDecideAlces_AcceptByWeight(t *testing.T) {
	fx := &fakeAlces{}
	gc := gcWith([]gamestate.ExEquipSubStatus{{Status: 12, Step: 1}}, fx) // 当前 物贯 step1
	// 目标 物攻(2)，优先级 物贯(12)——非目标次级属性。
	r := newRun(gc, map[int]int{2: 1}, computeWeight(map[int]int{2: 1}, []int{12}))

	// 无满级目标，但物贯从 step1→step3，加权更优 → 采纳。
	pending := &apiexequip.AlcesPending{SerialID: 100, SubStatus: []apiexequip.SubStatus{{SlotNumber: 1, Status: 12, Step: 3}}}
	accept, err := r.decideAlces(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if !accept || len(fx.fixed) != 1 {
		t.Fatalf("加权更优应采纳，得 accept=%v fixed=%v", accept, fx.fixed)
	}
}

// --- getAchieved ---

func TestGetAchieved(t *testing.T) {
	gc := gcWith([]gamestate.ExEquipSubStatus{{Status: 2, Step: 5}, {Status: 2, Step: 3}}, &fakeAlces{})
	r := newRun(gc, map[int]int{2: 2}, nil) // 想要 2 条 物攻
	achMax, ach := r.getAchieved()
	if ach != 2 || achMax != 1 {
		t.Fatalf("2条物攻(1满级)：应 achieved=2 achievedMax=1，得 %d %d", ach, achMax)
	}
}

// --- 端到端：看属性 ---

func TestViewAttributes(t *testing.T) {
	snap := mdexequip.NewSnapshot(
		map[int]int{1: 5},         // ex_equipment_id 1 → 稀有度 彩
		map[int]int{1: 10},        // → group 10
		map[int]string{1: "测试彩装"}, // 名称
		nil,                       // 物品名
		map[int]map[int][5]int{10: {2: {10, 20, 30, 40, 50}}}, // group10 物攻(2) 各档值
	)
	gc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ExEquips: map[int]gamestate.ExEquip{
			100: {SerialID: 100, ExEquipmentID: 1, SubStatus: []gamestate.ExEquipSubStatus{{Status: 2, Step: 5}}},
			200: {SerialID: 200, ExEquipmentID: 2}, // 非 5 星（rarity 未知=0），应被过滤
		}},
		MD: fakeReader{ex: fakeMDExequip{snap: snap}},
	}
	res := moduletest.RunOne(gc, rainbowEnhance{}, map[string]any{"ex_equip_rainbow_enchance_action": "看属性"})
	if res.Status != automation.StatusOK {
		t.Fatalf("状态应 OK，得 %s（%v）", res.Status, res.Err)
	}
	joined := strings.Join(res.Log, "\n")
	if !strings.Contains(joined, "1件彩装") || !strings.Contains(joined, "100: 彩-测试彩装 物攻x0.50%") {
		t.Fatalf("看属性输出不符：\n%s", joined)
	}
}

func TestViewAttributes_NoRainbow(t *testing.T) {
	snap := mdexequip.NewSnapshot(map[int]int{1: 4}, nil, map[int]string{1: "粉装"}, nil, nil)
	gc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ExEquips: map[int]gamestate.ExEquip{
			100: {SerialID: 100, ExEquipmentID: 1},
		}},
		MD: fakeReader{ex: fakeMDExequip{snap: snap}},
	}
	res := moduletest.RunOne(gc, rainbowEnhance{}, map[string]any{"ex_equip_rainbow_enchance_action": "看属性"})
	if res.Status != automation.StatusSkip {
		t.Fatalf("无彩装应 Skip，得 %s", res.Status)
	}
}

// --- 依赖世界的参数候选 ---

// worldGC 是一个「有一件彩装(serial 100)、母数据只认得物攻/物贯两种副属性」的世界。
func worldGC() *moduletest.FakeClient {
	snap := mdexequip.NewSnapshot(
		map[int]int{1: 5, 2: 4},            // 1=彩装，2=粉装
		map[int]int{1: 10, 2: 20},          // → group
		map[int]string{1: "测试彩装", 2: "粉装"}, // 名称
		nil, // 物品名
		map[int]map[int][5]int{ // group→status→各档值
			10: {2: {10, 20, 30, 40, 50}, 12: {1, 2, 3, 4, 5}}, // 物攻(2)、物贯(12)
			20: {2: {10, 20, 30, 40, 50}},
		},
	)
	return &moduletest.FakeClient{
		State: &gamestate.PlayerState{ExEquips: map[int]gamestate.ExEquip{
			100: {SerialID: 100, ExEquipmentID: 1, SubStatus: []gamestate.ExEquipSubStatus{{Status: 2, Step: 5}}},
			200: {SerialID: 200, ExEquipmentID: 2}, // 粉装，不该出现在彩装候选里
		}},
		MD: fakeReader{ex: fakeMDExequip{snap: snap}},
	}
}

// TestCandidatesContract 是契约检查：每个无静态候选的 Choice 参数都须被 Candidates 覆盖。
// Registry.Register 只抓得住「整个接口没实现」，漏掉其中某一个参数要真解析一次才知道。
func TestCandidatesContract(t *testing.T) {
	if err := automation.CheckCandidates(context.Background(), worldGC(), rainbowEnhance{}); err != nil {
		t.Fatalf("参数候选不自洽: %v", err)
	}
}

// TestCandidates_FromWorld 检查候选确实来自世界而非硬编码：彩装取自玩家库存（且滤掉非彩装、
// 显示成人话），副属性取自母数据里【出现过的】属性。后者曾是一张手选的常量表，会随版本漂移。
func TestCandidates_FromWorld(t *testing.T) {
	cands, err := rainbowEnhance{}.Candidates(context.Background(), worldGC())
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}

	equips := cands["ex_equip_rainbow_enchance_id"]
	if len(equips) != 1 || equips[0].Value != "100" {
		t.Fatalf("彩装候选=%+v want 仅 serial 100", equips)
	}
	if !strings.Contains(equips[0].Label, "彩-测试彩装") {
		t.Fatalf("彩装候选应显示成人话，得 %q", equips[0].Label)
	}

	// 母数据只认得物攻/物贯，候选就该只有这两个——不多不少。
	rank := cands["ex_equip_rainbow_enhance_rank"]
	if len(rank) != 2 || rank[0].Value != "物攻" || rank[1].Value != "物贯" {
		t.Fatalf("副属性候选=%+v want [物攻 物贯]", rank)
	}
	// 炼成目标额外可选「任意」＝不指定该槽。
	target := cands["ex_equip_rainbow_enchance_sub_status_1"]
	if len(target) != 3 || target[0].Value != "任意" {
		t.Fatalf("炼成属性候选=%+v want 任意打头的 3 项", target)
	}
}

// TestEnhance_RejectsUnownedEquip 是这套机制的正题：选了一件【自己没有的】彩装，在跑起来之前
// 就被挡下。以前这个参数是不受约束的自由文本，非法值要一路走到炼成主流程里才发现。
func TestEnhance_RejectsUnownedEquip(t *testing.T) {
	res := moduletest.RunOne(worldGC(), rainbowEnhance{}, map[string]any{
		"ex_equip_rainbow_enchance_action": "炼成",
		"ex_equip_rainbow_enchance_id":     "999",
	})
	if res.Status != automation.StatusError {
		t.Fatalf("未持有的彩装应被挡下，得 %s", res.Status)
	}
	if !strings.Contains(res.Err.Error(), "配置无效") {
		t.Fatalf("应报配置无效（而非跑进主流程才失败），得 %v", res.Err)
	}
}

// TestEnhance_RejectsUnknownSubStatus 检查副属性同样按世界校验：母数据里没有的属性名（如本世界
// 没有的「魔攻」）不再蒙混过关。
func TestEnhance_RejectsUnknownSubStatus(t *testing.T) {
	res := moduletest.RunOne(worldGC(), rainbowEnhance{}, map[string]any{
		"ex_equip_rainbow_enchance_action":       "炼成",
		"ex_equip_rainbow_enchance_id":           "100",
		"ex_equip_rainbow_enchance_sub_status_1": "魔攻",
	})
	if res.Status != automation.StatusError || !strings.Contains(res.Err.Error(), "配置无效") {
		t.Fatalf("母数据里没有的副属性应被挡下，得 %s（%v）", res.Status, res.Err)
	}
}
