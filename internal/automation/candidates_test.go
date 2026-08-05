package automation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client"
)

// candModule 是带依赖世界候选的测试模块：candidates/err 可控，故各用例能单独构造出
// "解析失败""漏给候选""候选给了未声明参数"等情形。不触碰 client，Run 可传 nil。
type candModule struct {
	stubModule
	cands map[string][]Option
	err   error
}

func (c candModule) Candidates(context.Context, client.GameClient) (map[string][]Option, error) {
	return c.cands, c.err
}

// dynParam 是一个无静态候选的 Choice 参数——其候选只能由 Candidates 在世界已知时给出。
func dynParam(name string) Param {
	return Param{Name: name, Type: ParamChoice, Default: "", Description: name}
}

// TestBindCandidates_FillsBounds 检查解析出的候选被填进 Bounds.Choices，且'不改入参'——
// Bounds.Choices 是唯一的候选源，依赖世界的参数只是要到 gc 可用时才填得上。
func TestBindCandidates_FillsBounds(t *testing.T) {
	params := []Param{dynParam("equip")}
	cands := map[string][]Option{"equip": {{Value: "1", Label: "彩-珠光 #1"}, {Value: "2", Label: "彩-月华 #2"}}}

	out, err := bindCandidates(params, cands)
	if err != nil {
		t.Fatalf("bindCandidates: %v", err)
	}
	if got := out[0].Bounds.Choices; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("Choices=%v want [1 2]", got)
	}
	if params[0].Bounds.Choices != nil {
		t.Fatalf("入参被改动: %v", params[0].Bounds.Choices)
	}
}

// TestBindCandidates_UndeclaredParam 检查候选给了未声明的参数即响亮失败（多半是参数名拼错，
// 静默则该参数永远拿不到候选）。
func TestBindCandidates_UndeclaredParam(t *testing.T) {
	_, err := bindCandidates([]Param{dynParam("equip")}, map[string][]Option{"equpi": {{Value: "1"}}})
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("err=%v want 未声明的参数", err)
	}
}

// TestBindCandidates_MissingCandidates 检查 Choice 类参数既无静态候选、Candidates 又漏给它时
// 响亮失败——这正是 Registry.Register 抓不住的那一半（它只知道接口实现没实现）。
func TestBindCandidates_MissingCandidates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		param Param
	}{
		{"choice", dynParam("equip")},
		{"multichoice", Param{Name: "rank", Type: ParamMultiChoice}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 别的参数给了候选，唯独漏了它——即"实现了 Candidates 但漏掉某一个参数"。
			_, err := bindCandidates([]Param{tc.param, dynParam("other")},
				map[string][]Option{"other": {{Value: "x"}}})
			if err == nil || !strings.Contains(err.Error(), "未给出") {
				t.Fatalf("err=%v want 未给出候选", err)
			}
		})
	}
}

// TestBindCandidates_EmptyIsLegal 检查'给了空候选'合法：它表示世界里当前没有可选项（如新号
// 一件彩装都没有），与"压根没给"是两回事。此时无可校验，由模块自己的 Skip 守卫接管。
func TestBindCandidates_EmptyIsLegal(t *testing.T) {
	out, err := bindCandidates([]Param{dynParam("equip")}, map[string][]Option{"equip": {}})
	if err != nil {
		t.Fatalf("空候选应合法: %v", err)
	}
	if got := out[0].Bounds.Choices; len(got) != 0 {
		t.Fatalf("Choices=%v want 空", got)
	}
}

// TestBindCandidates_StaticUntouched 检查有静态候选的参数不受影响（模块无须为它们实现
// Candidates），且非 Choice 类参数无候选不算错。
func TestBindCandidates_StaticUntouched(t *testing.T) {
	params := []Param{
		{Name: "action", Type: ParamChoice, Bounds: Bounds{Choices: []string{"看", "做"}}},
		{Name: "count", Type: ParamInt},
		{Name: "note", Type: ParamString},
	}
	out, err := bindCandidates(params, nil)
	if err != nil {
		t.Fatalf("bindCandidates: %v", err)
	}
	if got := out[0].Bounds.Choices; len(got) != 2 || got[0] != "看" {
		t.Fatalf("静态候选被改: %v", got)
	}
}

// TestRunOne_ValidatesAgainstWorld 是这套机制的要点：校验按'当前世界'解析出的候选进行。
// 配置的合法性本就是相对世界而言的——同一个 "3"，世界里有就合法、没有就该在跑起来前被挡下。
func TestRunOne_ValidatesAgainstWorld(t *testing.T) {
	ran := false
	m := candModule{
		stubModule: stubModule{
			meta:   Meta{Name: "enhance"},
			params: []Param{dynParam("equip")},
			fn:     func(*RunContext) error { ran = true; return nil },
		},
		cands: map[string][]Option{"equip": {{Value: "1", Label: "彩-珠光 #1"}}},
	}
	reg := NewRegistry()
	reg.Register(m)

	res, err := Run(context.Background(), nil, reg, []Task{
		{Module: "enhance", Values: map[string]any{"equip": "1"}}, // 世界里有
		{Module: "enhance", Values: map[string]any{"equip": "3"}}, // 世界里没有
	}, nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res[0].Status != StatusOK {
		t.Fatalf("世界里有的值应通过: %+v", res[0])
	}
	if res[1].Status != StatusError || !strings.Contains(res[1].Err.Error(), "配置无效") {
		t.Fatalf("世界里没有的值应被挡下: %+v", res[1])
	}
	if !ran {
		t.Fatal("合法任务未执行")
	}
}

// TestRunOne_CandidatesError 检查候选解析失败被隔离成该任务的 StatusError（如母数据未启用），
// 而非崩掉整批。
func TestRunOne_CandidatesError(t *testing.T) {
	m := candModule{
		stubModule: stubModule{
			meta:   Meta{Name: "enhance"},
			params: []Param{dynParam("equip")},
			fn:     func(*RunContext) error { t.Fatal("候选解析失败时不应执行"); return nil },
		},
		err: errors.New("母数据未启用"),
	}
	reg := NewRegistry()
	reg.Register(m)

	res, _ := Run(context.Background(), nil, reg, []Task{{Module: "enhance"}}, nil, nil)
	if res[0].Status != StatusError || !strings.Contains(res[0].Err.Error(), "解析参数候选") {
		t.Fatalf("res=%+v want 解析参数候选失败", res[0])
	}
}

// TestCheckCandidates 检查契约助手：自洽的模块通过，漏给候选的模块被抓——后者正是每个模块
// 的 _test.go 该调它一次的理由。
func TestCheckCandidates(t *testing.T) {
	ok := candModule{
		stubModule: stubModule{meta: Meta{Name: "ok"}, params: []Param{dynParam("equip")}},
		cands:      map[string][]Option{"equip": {{Value: "1"}}},
	}
	if err := CheckCandidates(context.Background(), nil, ok); err != nil {
		t.Fatalf("自洽模块应通过: %v", err)
	}

	missing := candModule{
		stubModule: stubModule{meta: Meta{Name: "missing"}, params: []Param{dynParam("equip"), dynParam("slot")}},
		cands:      map[string][]Option{"equip": {{Value: "1"}}}, // 漏了 slot
	}
	if err := CheckCandidates(context.Background(), nil, missing); err == nil {
		t.Fatal("漏给候选的模块应被抓")
	}
}

// TestRegisterPanicsOnUnboundChoice 检查注册期这道关：声明了无候选的 Choice 却没实现
// Candidates，是装配 bug，必须在 DefaultRegistry() 被调到的第一刻就炸，而不是等到运行时静默
// 退回零校验。
func TestRegisterPanicsOnUnboundChoice(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("未实现 Candidates 却有无候选 Choice 参数，应 panic")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Candidates") {
			t.Fatalf("panic 消息应指明须实现 Candidates: %v", r)
		}
	}()
	NewRegistry().Register(stubModule{meta: Meta{Name: "bad"}, params: []Param{dynParam("equip")}})
}

// TestRegisterAcceptsBoundChoice 检查有静态候选的 Choice 参数无须实现 Candidates（绝大多数
// 模块的情形，不该被这道关误伤）。
func TestRegisterAcceptsBoundChoice(t *testing.T) {
	NewRegistry().Register(stubModule{meta: Meta{Name: "fine"}, params: []Param{
		{Name: "action", Type: ParamChoice, Bounds: Bounds{Choices: []string{"看", "做"}}},
	}})
}
