package session

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// fakeCred 是仅供 passRisk 测试用的凭据打桩：Header/APIRoot 满足 transport.New，
// DoCaptcha 返回脚本化结果并计数。
type fakeCred struct {
	captchaCalls int
	captchaErr   error
}

func (f *fakeCred) Login(ctx context.Context) (string, string, error) { return "u", "k", nil }
func (f *fakeCred) Header() map[string]string                         { return map[string]string{} }
func (f *fakeCred) APIRoot() string                                   { return "https://test.example/" }
func (f *fakeCred) PlatformID() string                                { return "2" }
func (f *fakeCred) ChannelID() string                                 { return "1" }
func (f *fakeCred) DoCaptcha(ctx context.Context) (*captcha.Result, error) {
	f.captchaCalls++
	if f.captchaErr != nil {
		return nil, f.captchaErr
	}
	return &captcha.Result{Challenge: "chal", Validate: "vali", Seccode: "vali|jordan"}, nil
}

// scriptedLogin 安装一个短路中间件：对 tool/sdk_login 请求按 riskSeq 逐次返回 is_risk，
// 并记录每次收到的请求，供断言票据字段。
func scriptedLogin(t *testing.T, c *transport.Client, riskSeq []bool, got *[]*sdk.ToolSdkLoginRequest) {
	t.Helper()
	i := 0
	c.Use(func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			r, ok := req.(*sdk.ToolSdkLoginRequest)
			if !ok {
				t.Fatalf("非预期请求类型 %T", req)
			}
			*got = append(*got, r)
			resp, ok := out.(*sdk.ToolSdkLoginResponse)
			if !ok {
				t.Fatalf("非预期响应类型 %T", out)
			}
			risk := true
			if i < len(riskSeq) {
				risk = riskSeq[i]
			}
			i++
			resp.IsRisk = risk
			return protocol.ResponseHeader{}, nil
		}
	})
}

func TestPassRiskSucceedsAfterCaptcha(t *testing.T) {
	cred := &fakeCred{}
	c := transport.New(cred)
	var got []*sdk.ToolSdkLoginRequest
	// 前两轮仍 is_risk，第三轮通过。
	scriptedLogin(t, c, []bool{true, true, false}, &got)

	if err := passRisk(context.Background(), c, cred, "u", "k", nil); err != nil {
		t.Fatalf("passRisk 应成功，得到 %v", err)
	}
	if cred.captchaCalls != 3 {
		t.Errorf("DoCaptcha 调用 %d 次，期望 3", cred.captchaCalls)
	}
	if len(got) != 3 {
		t.Fatalf("重登请求 %d 次，期望 3", len(got))
	}
	// 断言票据字段（取第一次）。
	r := got[0]
	if r.Challenge == nil || *r.Challenge != "chal" {
		t.Errorf("challenge=%v，期望 chal", r.Challenge)
	}
	if r.Validate == nil || *r.Validate != "vali" {
		t.Errorf("validate=%v，期望 vali", r.Validate)
	}
	if r.Seccode == nil || *r.Seccode != "vali|jordan" {
		t.Errorf("seccode=%v，期望 vali|jordan", r.Seccode)
	}
	if r.CaptchaType == nil || *r.CaptchaType != "1" {
		t.Errorf("captcha_type=%v，期望 1", r.CaptchaType)
	}
	if r.ImageToken == nil || *r.ImageToken != "" {
		t.Errorf("image_token=%v，期望空串", r.ImageToken)
	}
	if r.CaptchaCode == nil || *r.CaptchaCode != "" {
		t.Errorf("captcha_code=%v，期望空串", r.CaptchaCode)
	}
	if r.UID != "u" || r.AccessKey != "k" || r.Platform != "2" || r.ChannelID != "1" {
		t.Errorf("四要素错误：%+v", r)
	}
}

func TestPassRiskExhaustsAttempts(t *testing.T) {
	cred := &fakeCred{}
	c := transport.New(cred)
	var got []*sdk.ToolSdkLoginRequest
	scriptedLogin(t, c, nil, &got) // 恒 is_risk

	err := passRisk(context.Background(), c, cred, "u", "k", nil)
	if err == nil {
		t.Fatal("恒风控应返回错误")
	}
	var re *gameerr.RiskError
	if !errors.As(err, &re) {
		t.Fatalf("应为 *gameerr.RiskError，得到 %T", err)
	}
	if re.Attempts != maxRiskAttempts {
		t.Errorf("RiskError.Attempts=%d，期望 %d", re.Attempts, maxRiskAttempts)
	}
	if cred.captchaCalls != maxRiskAttempts {
		t.Errorf("DoCaptcha 调用 %d 次，期望 %d", cred.captchaCalls, maxRiskAttempts)
	}
}

func TestPassRiskCaptchaError(t *testing.T) {
	sentinel := errors.New("solve boom")
	cred := &fakeCred{captchaErr: sentinel}
	c := transport.New(cred)
	var got []*sdk.ToolSdkLoginRequest
	scriptedLogin(t, c, nil, &got)

	err := passRisk(context.Background(), c, cred, "u", "k", nil)
	if err == nil {
		t.Fatal("求解失败应返回错误")
	}
	// RiskError 须保留底层成因，便于外壳诊断。
	if !errors.Is(err, sentinel) {
		t.Errorf("错误应可 errors.Is 命中底层求解错误，得到 %v", err)
	}
	var re *gameerr.RiskError
	if !errors.As(err, &re) {
		t.Errorf("应为 *gameerr.RiskError，得到 %T", err)
	}
	if len(got) != 0 {
		t.Errorf("求解失败不应发出重登请求，实际 %d 次", len(got))
	}
}

// TestPassRiskNoSolverHardFails 覆盖设计决策：未注入求解器时 is_risk 硬失败——
// 返回可诊断的 RiskError（errors.Is 命中 captcha.ErrNoSolver）、不发任何重登。
func TestPassRiskNoSolverHardFails(t *testing.T) {
	cred := &fakeCred{captchaErr: captcha.ErrNoSolver}
	c := transport.New(cred)
	var got []*sdk.ToolSdkLoginRequest
	scriptedLogin(t, c, nil, &got)

	err := passRisk(context.Background(), c, cred, "u", "k", nil)
	if err == nil {
		t.Fatal("无求解器应硬失败")
	}
	if !errors.Is(err, captcha.ErrNoSolver) {
		t.Errorf("错误应可 errors.Is 命中 captcha.ErrNoSolver，得到 %v", err)
	}
	var re *gameerr.RiskError
	if !errors.As(err, &re) {
		t.Fatalf("应为 *gameerr.RiskError，得到 %T", err)
	}
	if len(got) != 0 {
		t.Errorf("硬失败不应发出重登请求，实际 %d 次", len(got))
	}
	if cred.captchaCalls != 1 {
		t.Errorf("DoCaptcha 调用 %d 次，期望 1", cred.captchaCalls)
	}
}

// TestPassRiskThreadsInitialPayload 覆盖「无求解器（mobile）」路径：DoCaptcha 首轮即失败、不发
// 重登，故透出的 RiskError.Payload 应为传入的首个风控响应载荷（原样保留），且被内联进错误消息。
func TestPassRiskThreadsInitialPayload(t *testing.T) {
	cred := &fakeCred{captchaErr: captcha.ErrNoSolver}
	c := transport.New(cred)
	var got []*sdk.ToolSdkLoginRequest
	scriptedLogin(t, c, nil, &got)

	initial := map[string]any{"risk_type": "device", "n": int64(7)}
	err := passRisk(context.Background(), c, cred, "u", "k", initial)

	var re *gameerr.RiskError
	if !errors.As(err, &re) {
		t.Fatalf("应为 *gameerr.RiskError，得到 %T", err)
	}
	if re.Attempts != 0 {
		t.Errorf("无求解器应在首轮失败，Attempts=%d 期望 0", re.Attempts)
	}
	if !reflect.DeepEqual(re.Payload, initial) {
		t.Errorf("Payload=%v，期望原样保留首个风控载荷 %v", re.Payload, initial)
	}
	if !strings.Contains(re.Error(), "风控响应载荷") {
		t.Errorf("错误消息应内联风控载荷，得到 %q", re.Error())
	}
}

// TestPassRiskThreadsReloginPayload 覆盖「求解后仍风控直至耗尽」：透出的 Payload 应刷新为最近
// 一次重登风控响应的 Extra（而非最初传入的 nil）。
func TestPassRiskThreadsReloginPayload(t *testing.T) {
	cred := &fakeCred{}
	c := transport.New(cred)
	// 每次 tool/sdk_login 都回 is_risk，并在响应上挂一个未知载荷。
	c.Use(func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			resp, ok := out.(*sdk.ToolSdkLoginResponse)
			if !ok {
				t.Fatalf("非预期响应类型 %T", out)
			}
			resp.IsRisk = true
			resp.Extra = map[string]any{"round_marker": "x"}
			return protocol.ResponseHeader{}, nil
		}
	})

	err := passRisk(context.Background(), c, cred, "u", "k", nil)
	var re *gameerr.RiskError
	if !errors.As(err, &re) {
		t.Fatalf("应为 *gameerr.RiskError，得到 %T", err)
	}
	if want := map[string]any{"round_marker": "x"}; !reflect.DeepEqual(re.Payload, want) {
		t.Errorf("Payload=%v，期望刷新为重登载荷 %v", re.Payload, want)
	}
}

// scriptedSequence 安装一个短路中间件：记录整条登录序列的请求 URL，并按类型填最小可用响应。
func scriptedSequence(t *testing.T, c *transport.Client, urls *[]string) {
	t.Helper()
	c.Use(func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			*urls = append(*urls, req.URL().Path)
			switch o := out.(type) {
			case *sdk.SourceIniIndexResponse:
				o.Server = []string{"test.example"}
			case *sdk.CheckGameStartResponse:
				o.NowTutorial = true
			}
			return protocol.ResponseHeader{}, nil
		}
	})
}

// 登录序列锁定在这六步。权威客户端在其后还有 daily_task/top 与 unit_role/gacha_index，
// 本库有意不发（见 Login 的说明）——若哪天补回，这里会红，提醒同步更新。
func TestLoginSequence(t *testing.T) {
	c := transport.New(&fakeCred{})
	var urls []string
	scriptedSequence(t, c, &urls)
	if err := Login(context.Background(), c, &fakeCred{}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"source_ini/index", "source_ini/get_maintenance_status", "tool/sdk_login",
		"check/game_start", "load/index", "home/index",
	}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("登录序列不符\n got: %v\nwant: %v", urls, want)
	}
}
