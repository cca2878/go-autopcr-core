package client

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// --- 测试替身 ---

type fakeReq struct{ protocol.RequestBase }

func (*fakeReq) URL() *url.URL { return protocol.MustRelURL("test/fake") }

// fakeResp 内嵌 ResponseBase，用来验证重发前 out 被清零（残留 server_error 会污染重发结果）。
type fakeResp struct {
	protocol.ResponseBase
	Value int
}

func apiErr(code, status int, msg string) error {
	return &gameerr.APIError{Message: msg, Status: status, ResultCode: code}
}

// newGuard 造一个记录重登次数的 guard。
func newGuard() (*sessionGuard, *int) {
	logins := 0
	g := &sessionGuard{
		login:  func(context.Context) error { logins++; return nil },
		logger: slog.New(slog.DiscardHandler),
	}
	return g, &logins
}

// runGuard 让 guard 包住一个按脚本作答的处理器，返回处理器实际被调用的次数。
func runGuard(ctx context.Context, g *sessionGuard, out any, script []error) (int, error) {
	calls := 0
	h := g.middleware()(func(_ context.Context, _ protocol.Request, o any) (protocol.ResponseHeader, error) {
		i := calls
		calls++
		if i >= len(script) {
			return protocol.ResponseHeader{}, nil
		}
		if err := script[i]; err != nil {
			// 复刻真实传输层：业务错误也会把 server_error 解进 out。
			if r, ok := o.(*fakeResp); ok {
				r.ServerError = &protocol.ErrorInfo{Message: err.Error()}
			}
			return protocol.ResponseHeader{}, err
		}
		return protocol.ResponseHeader{}, nil
	})
	_, err := h(ctx, &fakeReq{}, out)
	return calls, err
}

// tolerantCtx 是「模块声明容忍断点」的 ctx——自愈重发只在这一档下发生。
func tolerantCtx() context.Context {
	return WithBreakPolicy(context.Background(), BreakIgnore)
}

// --- 判定 ---

func TestClassifySession(t *testing.T) {
	const title = "发生错误，请回到标题界面重新登录"
	cases := []struct {
		name string
		err  error
		want sessionFault
	}{
		{"顶号 6002 可重发", apiErr(6002, 1, title), faultRetry},
		{"会话错误 4 可重发", apiErr(4, 1, title), faultRetry},
		{"status=3 只标记", apiErr(9999, 3, "会话已失效"), faultStale},
		{"未知码靠 message 兜底", apiErr(9999, 1, title), faultStale},
		// 107 与需要重登的错误【共用同一句 message】，故必须靠错误码豁免：
		// 同一 access_key 重跑登录必然再失败，重登纯属空转。
		{"107 假凭据不重登", apiErr(107, 1, title), faultNone},
		{"普通业务错误", apiErr(203, 1, "体力不足"), faultNone},
		{"非 API 错误", errors.New("boom"), faultNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, got := classifySession(c.err); got != c.want {
				t.Errorf("classifySession = %v，应为 %v", got, c.want)
			}
		})
	}
}

// --- 中间件 ---

// TestRelogin_RetriesAfterKick 断言【容忍断点的模块】被顶号后自动重登并原样重发，
// 模块层看到的是成功，而不是一个它无从处理的会话错误。
func TestRelogin_RetriesAfterKick(t *testing.T) {
	g, logins := newGuard()
	out := &fakeResp{}

	calls, err := runGuard(tolerantCtx(), g, out, []error{apiErr(6002, 1, "请回到标题界面")})

	if err != nil {
		t.Fatalf("重登后重发应成功，得 %v", err)
	}
	if *logins != 1 {
		t.Errorf("应重登 1 次，得 %d", *logins)
	}
	if calls != 2 {
		t.Errorf("请求应发出 2 次（原始 + 重发），得 %d", calls)
	}
	// 重发前必须清零 out：否则上一轮的 server_error 残留会让成功响应被再次判为业务错误。
	if out.ServerError != nil {
		t.Errorf("重发前应清零响应载体，却残留 server_error=%+v", out.ServerError)
	}
}

// TestRelogin_MarkerOnlyDoesNotResend 断言靠 message 兜底命中的未知错误码【不重发】：
// 无从判断服务端是否已部分执行请求，盲目重发可能重复扣资源。
func TestRelogin_MarkerOnlyDoesNotResend(t *testing.T) {
	g, logins := newGuard()

	calls, err := runGuard(tolerantCtx(), g, &fakeResp{},
		[]error{apiErr(9999, 1, "数据异常，请回到标题界面")})

	if err == nil {
		t.Fatal("错误应原样上抛")
	}
	if calls != 1 {
		t.Errorf("不应重发，请求次数应为 1，得 %d", calls)
	}
	if *logins != 0 {
		t.Errorf("本次请求不应触发重登，得 %d 次", *logins)
	}
	if !g.stale {
		t.Error("会话应被标记失效，以便下次请求前重登")
	}

	// 惰性重登：下一次请求前才补上，且随后正常放行。
	calls, err = runGuard(tolerantCtx(), g, &fakeResp{}, nil)
	if err != nil {
		t.Fatalf("重登后应正常放行，得 %v", err)
	}
	if *logins != 1 || calls != 1 {
		t.Errorf("下次请求前应重登 1 次并发出 1 次请求，得 logins=%d calls=%d", *logins, calls)
	}
}

// TestRelogin_DefaultPolicyFailsFast 是断点语义的核心：默认（未声明策略）下，会话失效
// 【当场】变成 SessionBreakError 上抛，不悄悄重发。自愈修得了会话，修不了调用方局部变量
// 里那份已过时的世界快照——让它在断点处 unwind，好过带着旧结论继续往下写。
func TestRelogin_DefaultPolicyFailsFast(t *testing.T) {
	for _, p := range []struct {
		name   string
		policy BreakPolicy
	}{
		{"默认(未声明)", BreakAbort}, // 走 context.Background()，不放策略
		{"显式 abort", BreakAbort},
		{"restart(传输层同 abort，重跑由运行器做)", BreakRestart},
	} {
		t.Run(p.name, func(t *testing.T) {
			g, logins := newGuard()
			ctx := context.Background()
			if p.name != "默认(未声明)" {
				ctx = WithBreakPolicy(ctx, p.policy)
			}
			cause := apiErr(6002, 1, "请回到标题界面")

			calls, err := runGuard(ctx, g, &fakeResp{}, []error{cause})

			var br *gameerr.SessionBreakError
			if !errors.As(err, &br) {
				t.Fatalf("应返回 SessionBreakError，得 %v", err)
			}
			if !errors.Is(err, cause) {
				t.Error("应保留原始成因，便于诊断真正发生了什么")
			}
			if calls != 1 {
				t.Errorf("不应重发，得 %d 次", calls)
			}
			if *logins != 0 {
				t.Errorf("本次调用不应就地重登（下次请求前才补），得 %d 次", *logins)
			}
			// 会话仍然要修——不修后续请求全废；分歧只在这一次调用怎么办。
			if !g.stale {
				t.Error("会话应被标记失效，下次请求前重登")
			}
		})
	}
}

// TestRelogin_DefaultRecoversNextRequest 是用户最关心的保证在最小尺度上的复现：默认档下
// 被顶号，当次请求失败（SessionBreakError），但【下一次请求会自己重登恢复，无需外部主动登录】。
//
// 这正是 GUI 的用法流程——同一个持久 Session 反复 Run：某个模块中途顶号→该任务失败，用户
// 再点运行任何模块，下一次请求前 ensure 自动补上重登。绝不会卡在「必须先手动重新登录」。
func TestRelogin_DefaultRecoversNextRequest(t *testing.T) {
	g, logins := newGuard()
	ctx := context.Background() // 默认档（未声明策略）

	// 第一次请求：执行中被顶号 → 当次失败，会话标记失效，但不就地重登。
	calls, err := runGuard(ctx, g, &fakeResp{}, []error{apiErr(6002, 1, "请回到标题界面")})
	if _, ok := gameerr.AsSessionBreak(err); !ok {
		t.Fatalf("首次应因顶号返回 SessionBreakError，得 %v", err)
	}
	if calls != 1 || *logins != 0 {
		t.Fatalf("首次不应就地重登，得 calls=%d logins=%d", calls, *logins)
	}
	if !g.stale {
		t.Fatal("会话应被标记失效")
	}

	// 第二次请求：无需任何外部干预，ensure 自动重登并放行——这就是「不必主动 relogin」。
	calls, err = runGuard(ctx, g, &fakeResp{}, nil)
	if err != nil {
		t.Fatalf("下一次请求应自动恢复，得 %v", err)
	}
	if *logins != 1 {
		t.Errorf("下一次请求前应自动重登 1 次，得 %d", *logins)
	}
	if calls != 1 {
		t.Errorf("重登后请求应正常发出，得 %d 次", calls)
	}
	if g.stale {
		t.Error("恢复后不应再残留失效标记")
	}
}

// TestRelogin_BadCredentialNoRelogin 断言 107 不触发重登（它的 message 同样含特征串）。
func TestRelogin_BadCredentialNoRelogin(t *testing.T) {
	g, logins := newGuard()

	calls, err := runGuard(context.Background(), g, &fakeResp{},
		[]error{apiErr(107, 1, "请回到标题界面")})

	if err == nil {
		t.Fatal("假凭据错误应上抛")
	}
	if calls != 1 || *logins != 0 {
		t.Errorf("不应重发或重登，得 calls=%d logins=%d", calls, *logins)
	}
	if g.stale {
		t.Error("假凭据不应把会话标记为可重登失效")
	}
}

// TestRelogin_BudgetExhausted 断言持续失败时重发有上限，不会无限打服务器。
func TestRelogin_BudgetExhausted(t *testing.T) {
	g, logins := newGuard()
	always := make([]error, maxReloginRetries+5)
	for i := range always {
		always[i] = apiErr(6002, 1, "请回到标题界面")
	}

	calls, err := runGuard(tolerantCtx(), g, &fakeResp{}, always)

	if err == nil {
		t.Fatal("重发耗尽后错误应上抛")
	}
	if calls != maxReloginRetries+1 {
		t.Errorf("请求应发出 %d 次，得 %d", maxReloginRetries+1, calls)
	}
	if *logins != maxReloginRetries {
		t.Errorf("应重登 %d 次，得 %d", maxReloginRetries, *logins)
	}
}

// TestRelogin_SkipsLoginSequence 断言登录序列自身的请求绕过本中间件——否则登录里
// 的一次会话错误会再触发重登，无限递归。
func TestRelogin_SkipsLoginSequence(t *testing.T) {
	g, logins := newGuard()
	g.invalidate() // 即便处于待重登状态

	calls, err := runGuard(markRelogin(context.Background()), g, &fakeResp{},
		[]error{apiErr(6002, 1, "请回到标题界面")})

	if err == nil {
		t.Fatal("登录序列内的错误应原样上抛")
	}
	if calls != 1 {
		t.Errorf("不应重发，得 %d 次", calls)
	}
	if *logins != 0 {
		t.Errorf("登录序列内不应递归重登，得 %d 次", *logins)
	}
}

// TestRelogin_LoginFailurePropagates 断言重登本身失败时错误上抛，且不吞掉——
// 否则调用方会看到一个与真实成因无关的错误。
func TestRelogin_LoginFailurePropagates(t *testing.T) {
	boom := errors.New("重登失败")
	g := &sessionGuard{
		login:  func(context.Context) error { return boom },
		logger: slog.New(slog.DiscardHandler),
	}
	g.invalidate()

	calls, err := runGuard(context.Background(), g, &fakeResp{}, nil)

	if !errors.Is(err, boom) {
		t.Fatalf("应上抛重登错误，得 %v", err)
	}
	if calls != 0 {
		t.Errorf("重登失败时不应发出请求，得 %d 次", calls)
	}
	if !g.stale {
		t.Error("重登失败后应保持失效标记，下次仍需重登")
	}
}
