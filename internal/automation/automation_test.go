package automation

import (
	"context"
	"errors"
	"testing"

	"github.com/cca2878/go-autopcr/internal/client"
)

// stubModule 是仅用于测试的模块：不触碰 client，故 Run 可传 nil GameClient。
type stubModule struct {
	meta   Meta
	params []Param
	fn     func(rc *RunContext) error
}

func (s stubModule) Meta() Meta      { return s.meta }
func (s stubModule) Params() []Param { return s.params }
func (s stubModule) Run(_ context.Context, _ client.GameClient, rc *RunContext) error {
	return s.fn(rc)
}

func TestRunStatuses(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubModule{meta: Meta{Name: "ok"}, fn: func(rc *RunContext) error { rc.Logf("done"); return nil }})
	reg.Register(stubModule{meta: Meta{Name: "skip"}, fn: func(rc *RunContext) error { return Skip("体力不足") }})
	reg.Register(stubModule{meta: Meta{Name: "err"}, fn: func(rc *RunContext) error { return errors.New("boom") }})
	tasks := []Task{{Module: "ok"}, {Module: "skip"}, {Module: "err"}, {Module: "ghost"}}

	res, err := Run(context.Background(), nil, reg, tasks, nil) // gc=nil：stub 不使用它，验证 Runner 与客户端解耦
	if err != nil {
		t.Fatalf("未取消不应返回 error: %v", err)
	}
	if len(res) != 4 {
		t.Fatalf("结果数=%d want 4", len(res))
	}
	if res[0].Status != StatusOK || len(res[0].Log) != 1 {
		t.Fatalf("ok: %+v", res[0])
	}
	if res[1].Status != StatusSkip {
		t.Fatalf("skip 状态=%s", res[1].Status)
	}
	if len(res[1].Log) == 0 || res[1].Log[len(res[1].Log)-1] != "体力不足" {
		t.Fatalf("skip 未记录原因: %+v", res[1].Log)
	}
	if res[2].Status != StatusError || res[2].Err == nil {
		t.Fatalf("err: %+v", res[2])
	}
	if res[3].Status != StatusError || res[3].Err == nil { // 未知模块名
		t.Fatalf("ghost: %+v", res[3])
	}
}

// TestPerInstanceConfig 验证同一模块在一批内出现两次、各带不同配置（点 3）。
func TestPerInstanceConfig(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubModule{
		meta:   Meta{Name: "echo"},
		params: []Param{{Name: "n", Type: ParamInt, Default: 0}},
		fn:     func(rc *RunContext) error { rc.Logf("n=%d", rc.Int("n")); return nil },
	})
	tasks := []Task{
		{Module: "echo", Values: map[string]any{"n": 1}},
		{Module: "echo", Values: map[string]any{"n": 2}},
		{Module: "echo"}, // 无值→默认 0
	}
	res, _ := Run(context.Background(), nil, reg, tasks, nil)
	got := []string{res[0].Log[0], res[1].Log[0], res[2].Log[0]}
	want := []string{"n=1", "n=2", "n=0"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个实例 log=%q want %q", i, got[i], want[i])
		}
	}
}

// TestValidateBounds 验证通用边界（类型/范围/choice/未知参数）。
func TestValidateBounds(t *testing.T) {
	min, max := 1, 10
	params := []Param{
		{Name: "count", Type: ParamInt, Bounds: Bounds{Min: &min, Max: &max}},
		{Name: "mode", Type: ParamChoice, Bounds: Bounds{Choices: []string{"a", "b"}}},
	}
	ok := []map[string]any{{"count": 5}, {"count": float64(10)}, {"mode": "b"}, nil}
	for _, v := range ok {
		if err := Validate(params, v); err != nil {
			t.Fatalf("Validate(%v) 应通过，得 %v", v, err)
		}
	}
	bad := []map[string]any{
		{"count": 0},   // < Min
		{"count": 11},  // > Max
		{"count": "x"}, // 类型
		{"mode": "z"},  // 不在 choices
		{"unknown": 1}, // 未声明
	}
	for _, v := range bad {
		if err := Validate(params, v); err == nil {
			t.Fatalf("Validate(%v) 应报错", v)
		}
	}
}

// runOne 校验失败应产出 error 结果而非崩溃。
func TestRunRejectsInvalidConfig(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubModule{
		meta:   Meta{Name: "x"},
		params: []Param{{Name: "mode", Type: ParamChoice, Bounds: Bounds{Choices: []string{"a"}}}},
		fn:     func(rc *RunContext) error { t.Fatal("非法配置不应执行 Run"); return nil },
	})
	res, _ := Run(context.Background(), nil, reg, []Task{{Module: "x", Values: map[string]any{"mode": "z"}}}, nil)
	if res[0].Status != StatusError || res[0].Err == nil {
		t.Fatalf("非法配置应 error: %+v", res[0])
	}
}

func TestRegistrySelect(t *testing.T) {
	r := NewRegistry()
	r.Register(stubModule{meta: Meta{Name: "a"}})
	r.Register(stubModule{meta: Meta{Name: "b"}})

	if got := len(r.All()); got != 2 {
		t.Fatalf("All 数=%d want 2", got)
	}
	mods, unknown := r.Select("b", "zzz")
	if len(mods) != 1 || mods[0].Meta().Name != "b" {
		t.Fatalf("Select 命中错误: %+v", mods)
	}
	if len(unknown) != 1 || unknown[0] != "zzz" {
		t.Fatalf("未知名字未报告: %+v", unknown)
	}
}

func TestRegistryGrouping(t *testing.T) {
	r := NewRegistry()
	r.Register(stubModule{meta: Meta{Name: "a", Category: "x"}})
	r.Register(stubModule{meta: Meta{Name: "b", Category: "y"}})
	r.Register(stubModule{meta: Meta{Name: "c", Category: "x"}})
	r.RegisterPreset(Preset{Name: "p", Modules: []string{"c", "a"}})

	if cat := r.ByCategory("x"); len(cat) != 2 || cat[0].Meta().Name != "a" || cat[1].Meta().Name != "c" {
		t.Fatalf("ByCategory 顺序/命中错误: %+v", cat)
	}
	mods, ok, unknown := r.Preset("p")
	if !ok || len(mods) != 2 || mods[0].Meta().Name != "c" || len(unknown) != 0 {
		t.Fatalf("Preset 解析错误: mods=%+v ok=%v unknown=%v", mods, ok, unknown)
	}
	if _, ok, _ := r.Preset("nope"); ok {
		t.Fatal("未知预设应返回 ok=false")
	}
}

func TestConfigResolution(t *testing.T) {
	params := []Param{
		{Name: "flag", Type: ParamBool, Default: true},
		{Name: "count", Type: ParamInt, Default: 3},
		{Name: "label", Type: ParamString, Default: "x"},
	}
	// 无外部值 → 用默认。
	def := resolve(params, nil)
	if !def.Bool("flag") || def.Int("count") != 3 || def.String("label") != "x" {
		t.Fatalf("默认解析错误: %v %d %q", def.Bool("flag"), def.Int("count"), def.String("label"))
	}
	// 外部覆盖，含 JSON 数字的 float64。
	ov := resolve(params, map[string]any{"flag": false, "count": float64(9), "label": "y"})
	if ov.Bool("flag") || ov.Int("count") != 9 || ov.String("label") != "y" {
		t.Fatalf("覆盖解析错误: %v %d %q", ov.Bool("flag"), ov.Int("count"), ov.String("label"))
	}
}

// TestRunObserverEvents 验证进度端口：事件序列、Finished 携带结果、Log 副本隔离、以及
// observer 不影响返回结果（确定性）。
func TestRunObserverEvents(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubModule{meta: Meta{Name: "a", Title: "甲"}, fn: func(rc *RunContext) error { rc.Logf("甲done"); return nil }})
	reg.Register(stubModule{meta: Meta{Name: "b", Title: "乙"}, fn: func(rc *RunContext) error { return Skip("跳过") }})
	tasks := []Task{{Module: "a"}, {Module: "b"}}

	var events []Event
	res, err := Run(context.Background(), nil, reg, tasks, func(ev Event) { events = append(events, ev) })
	if err != nil {
		t.Fatalf("未取消不应返回 error: %v", err)
	}

	// 期望序列：Started(0),Finished(0),Started(1),Finished(1)，Total 恒为 2。
	want := []struct {
		phase Phase
		idx   int
	}{{PhaseStarted, 0}, {PhaseFinished, 0}, {PhaseStarted, 1}, {PhaseFinished, 1}}
	if len(events) != len(want) {
		t.Fatalf("事件数=%d want %d: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		if events[i].Phase != w.phase || events[i].Index != w.idx || events[i].Total != 2 {
			t.Fatalf("事件[%d]=%+v want phase=%v idx=%d total=2", i, events[i], w.phase, w.idx)
		}
	}
	// Finished 携带对应结果状态。
	if events[1].Result.Status != StatusOK || events[3].Result.Status != StatusSkip {
		t.Fatalf("Finished 结果状态错误: %v %v", events[1].Result.Status, events[3].Result.Status)
	}
	// 事件里的 Result.Log 是副本：篡改它不影响返回结果。
	events[1].Result.Log[0] = "TAMPERED"
	if res[0].Log[0] == "TAMPERED" {
		t.Fatal("事件 Result.Log 未与返回结果隔离")
	}
	// 确定性：nil observer 与有 observer 返回一致。
	res2, _ := Run(context.Background(), nil, reg, tasks, nil)
	if len(res) != len(res2) || res[0].Status != res2[0].Status || res[1].Status != res2[1].Status {
		t.Fatalf("observer 影响了返回结果: %+v vs %+v", res, res2)
	}
}

// TestRunCancelStopsAtBoundary 验证 B1 边界取消：取消后不再调度后续任务，返回已完成部分 +
// context.Canceled。
func TestRunCancelStopsAtBoundary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reg := NewRegistry()
	reg.Register(stubModule{meta: Meta{Name: "a"}, fn: func(rc *RunContext) error { cancel(); return nil }}) // 跑完即取消
	reg.Register(stubModule{meta: Meta{Name: "b"}, fn: func(rc *RunContext) error { t.Fatal("取消后不应执行后续任务"); return nil }})

	res, err := Run(ctx, nil, reg, []Task{{Module: "a"}, {Module: "b"}}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，得 %v", err)
	}
	if len(res) != 1 || res[0].Meta.Name != "a" || res[0].Status != StatusOK {
		t.Fatalf("应只含已完成的 a: %+v", res)
	}
}

// TestRunCancelDuringTaskNotError 验证任务执行途中被取消（在途请求失败）归为取消而非失败：
// 该任务不计入结果、后续不执行。
func TestRunCancelDuringTaskNotError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reg := NewRegistry()
	reg.Register(stubModule{meta: Meta{Name: "a"}, fn: func(rc *RunContext) error { return nil }})
	reg.Register(stubModule{meta: Meta{Name: "b"}, fn: func(rc *RunContext) error {
		cancel()                // 模拟执行途中被取消
		return context.Canceled // 在途请求因 ctx 取消而失败
	}})
	reg.Register(stubModule{meta: Meta{Name: "c"}, fn: func(rc *RunContext) error { t.Fatal("取消后不应执行 c"); return nil }})

	res, err := Run(ctx, nil, reg, []Task{{Module: "a"}, {Module: "b"}, {Module: "c"}}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，得 %v", err)
	}
	if len(res) != 1 || res[0].Meta.Name != "a" {
		t.Fatalf("被取消的 b 不应记为结果、c 不应执行；得 %+v", res)
	}
}
