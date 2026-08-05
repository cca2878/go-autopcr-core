package automation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
)

// probeModule 记录每次运行时'传输层会看到的'断点策略，并可脚本化地撞断点。
// 它'不'实现 SessionAware——那正是绝大多数模块的样子（默认档）。
type probeModule struct {
	name string
	// breakUntil 表示前 n 次运行都撞上断点（0＝从不断点）。
	breakUntil int
	runs       int
	seen       []client.BreakPolicy
}

func (m *probeModule) Meta() Meta    { return Meta{Name: m.name} }
func (*probeModule) Params() []Param { return nil }

func (m *probeModule) Run(ctx context.Context, _ client.GameClient, rc *RunContext) error {
	m.runs++
	m.seen = append(m.seen, client.BreakPolicyFrom(ctx))
	rc.Logf("第 %d 次运行", m.runs)
	if m.runs <= m.breakUntil {
		return &gameerr.SessionBreakError{Cause: errors.New("被顶号")}
	}
	return nil
}

// policyModule 是声明了策略的模块（多实现一个 SessionAware 方法，别无差别）。
type policyModule struct {
	probeModule
	policy BreakPolicy
}

func (m *policyModule) OnSessionBreak() BreakPolicy { return m.policy }

func runModule(m Module) Result {
	reg := NewRegistry()
	reg.Register(m)
	res, _ := Run(context.Background(), nil, reg, []Task{{Module: m.Meta().Name}}, nil, nil)
	return res[0]
}

// TestSessionBreak_DefaultAborts 断言未声明策略的模块（＝绝大多数）撞上断点即失败、不重跑，
// 且措辞如实说明"可能已部分执行"——写请求到底发出去没有，我们确实不知道。
func TestSessionBreak_DefaultAborts(t *testing.T) {
	m := &probeModule{name: "plain", breakUntil: 1}

	res := runModule(m)

	if res.Status != StatusError {
		t.Fatalf("状态应为 error，得 %s", res.Status)
	}
	if m.runs != 1 {
		t.Errorf("默认档不应重跑，运行了 %d 次", m.runs)
	}
	if _, ok := gameerr.AsSessionBreak(res.Err); !ok {
		t.Fatalf("错误应可识别为会话断点（外壳据此提示用户），得 %v", res.Err)
	}
	if !strings.Contains(res.Err.Error(), "可能已部分执行") {
		t.Errorf("措辞应如实说明结果不可信，得 %v", res.Err)
	}
}

// TestSessionBreak_RestartRerunsOnce 断言声明可重跑的模块在断点后从头再跑一遍并成功，
// 且断点'前'那半程的日志保留——那半程可能已经写入过，用户要看得见。
func TestSessionBreak_RestartRerunsOnce(t *testing.T) {
	m := &policyModule{probeModule: probeModule{name: "restartable", breakUntil: 1}, policy: BreakRestart}

	res := runModule(m)

	if res.Status != StatusOK {
		t.Fatalf("重跑后应成功，得 %s（%v）", res.Status, res.Err)
	}
	if m.runs != 2 {
		t.Errorf("应重跑 1 次（共 2 次），得 %d", m.runs)
	}
	if len(res.Log) != 3 || res.Log[0] != "第 1 次运行" || res.Log[2] != "第 2 次运行" {
		t.Fatalf("应保留断点前的日志并接着记新一轮，得 %q", res.Log)
	}
	if !strings.Contains(res.Log[1], "从头重试") {
		t.Errorf("中间应有一行说明发生了重跑，得 %q", res.Log[1])
	}
}

// TestSessionBreak_RestartGivesUpAfterOne 断言重跑不会没完没了：持续被顶号时第二次断点即
// 放弃，否则每重跑一轮就把已有副作用再做一遍。
func TestSessionBreak_RestartGivesUpAfterOne(t *testing.T) {
	m := &policyModule{probeModule: probeModule{name: "always-broken", breakUntil: 99}, policy: BreakRestart}

	res := runModule(m)

	if res.Status != StatusError {
		t.Fatalf("持续断点应最终失败，得 %s", res.Status)
	}
	if m.runs != 2 {
		t.Errorf("应只重跑 1 次（共 2 次）后放弃，得 %d", m.runs)
	}
	if _, ok := gameerr.AsSessionBreak(res.Err); !ok {
		t.Errorf("最终错误仍应是会话断点，得 %v", res.Err)
	}
}

// TestSessionBreak_PolicyReachesTransport 断言模块声明的策略确实进了 ctx。这是整条链路的
// 关键一环：断点是在某次 gc 调用'里面'被发现的，策略传不到那里就当场中止不了。
func TestSessionBreak_PolicyReachesTransport(t *testing.T) {
	type probe struct {
		name  string
		m     Module
		probe *probeModule // 取回它这一轮看到的策略
		want  client.BreakPolicy
	}

	// 不实现 SessionAware 的模块（＝绝大多数）应拿到最保守的一档。
	plain := &probeModule{name: "plain"}
	cases := []probe{{"未声明即最保守", plain, plain, BreakAbort}}

	for _, p := range []struct {
		name   string
		policy BreakPolicy
	}{{"声明容忍", BreakIgnore}, {"声明重跑", BreakRestart}, {"显式中止", BreakAbort}} {
		m := &policyModule{probeModule: probeModule{name: p.name}, policy: p.policy}
		cases = append(cases, probe{p.name, m, &m.probeModule, p.policy})
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if res := runModule(c.m); res.Status != StatusOK {
				t.Fatalf("探针模块应正常结束，得 %s", res.Status)
			}
			if seen := c.probe.seen; len(seen) != 1 || seen[0] != c.want {
				t.Errorf("传输层看到的策略=%v，应为 %v", seen, c.want)
			}
		})
	}
}
