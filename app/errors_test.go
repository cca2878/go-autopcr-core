// 本文件【有意】放在外部测试包 app_test 里：它只能碰 app 导出的符号，因而是「外壳视角」的
// 活体检验——门面漏掉哪个类型或哨兵，这里第一个编译不过。core 之下全是 internal，外壳没有
// 第二条路可走。
package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cca2878/go-autopcr-core/app"
)

// 未登录就 Run 是外壳最容易犯的错，必须可判定——否则只能去匹配那句中文。
func TestRunBeforeLoginIsIdentifiable(t *testing.T) {
	s := app.NewSession(app.Dirs{Cache: t.TempDir()})
	defer func() { _ = s.Close() }()

	_, err := s.Run(context.Background(), []app.Task{{Module: "summary"}}, nil)
	if !errors.Is(err, app.ErrNotLoggedIn) {
		t.Fatalf("应命中 app.ErrNotLoggedIn，得到 %v", err)
	}
	if got := app.ClassifyError(err).Kind; got != app.ErrorMisuse {
		t.Errorf("ClassifyError = %v, want ErrorMisuse", got)
	}
}

// 外壳拿到错误的默认动作是先分类、再按需取细节。这两步都得在门面上走得通。
func TestClassifyAndInspectFromShellSide(t *testing.T) {
	apiErr := &app.APIError{Message: "体力不足", Status: 1, ResultCode: 204}

	if got := app.ClassifyError(apiErr).Kind; got != app.ErrorRejected {
		t.Fatalf("ClassifyError = %v, want ErrorRejected", got)
	}
	// 分类之后还要取得出 result_code——这正是只导出接口而不导出类型时做不到的事。
	var ae *app.APIError
	if !errors.As(error(apiErr), &ae) {
		t.Fatal("外壳应能 errors.As 到 app.APIError")
	}
	if ae.ResultCode != 204 || ae.Message != "体力不足" {
		t.Errorf("取到的字段不对：%+v", ae)
	}
}

func TestSessionBreakIsTransientAndInspectable(t *testing.T) {
	cause := errors.New("被顶号")
	err := error(&app.SessionBreakError{Cause: cause})

	if got := app.ClassifyError(err).Kind; got != app.ErrorTransient {
		t.Errorf("ClassifyError = %v, want ErrorTransient", got)
	}
	var sb *app.SessionBreakError
	if !errors.As(err, &sb) {
		t.Fatal("外壳应能 errors.As 到 app.SessionBreakError")
	}
	if !errors.Is(err, cause) {
		t.Error("应保留成因")
	}
}

// 维护中经 PanicError 上抛：它与普通业务错误同属 ErrorRejected，外壳往往要为它换一套提示，
// 故必须能单独认出来。
func TestPanicErrorDistinguishableFromAPIError(t *testing.T) {
	maintenance := error(&app.PanicError{Msg: "服务器维护中"})

	if got := app.ClassifyError(maintenance).Kind; got != app.ErrorRejected {
		t.Errorf("ClassifyError = %v, want ErrorRejected", got)
	}
	var pe *app.PanicError
	if !errors.As(maintenance, &pe) {
		t.Fatal("外壳应能 errors.As 到 app.PanicError")
	}
	var ae *app.APIError
	if errors.As(maintenance, &ae) {
		t.Error("PanicError 不应同时被认成 APIError——那样就分不出维护态了")
	}
}

func TestRiskErrorCarriesPayload(t *testing.T) {
	err := error(&app.RiskError{Attempts: 2, Cause: app.ErrNoSolver,
		Payload: map[string]any{"unknown_field": 1}})

	if got := app.ClassifyError(err).Kind; got != app.ErrorRejected {
		t.Errorf("ClassifyError = %v, want ErrorRejected", got)
	}
	// 没注入求解器这一情形要能单独认出来：它是外壳漏了装配，补上即可。
	if !errors.Is(err, app.ErrNoSolver) {
		t.Error("应能命中 app.ErrNoSolver")
	}
	var re *app.RiskError
	if !errors.As(err, &re) || len(re.Payload) == 0 {
		t.Error("外壳应能取到风控响应的未建模字段")
	}
}

// 非本库的错误不能被硬塞进某个类别，否则外壳会按错误的大方向处置它。
func TestForeignErrorIsUnknown(t *testing.T) {
	got := app.ClassifyError(errors.New("来自外壳自己"))
	if got.Kind != app.ErrorUnknown || got.Domain != app.ErrorFromUnknown {
		t.Errorf("ClassifyError = %v, want 两维皆 Unknown", got)
	}
}

// ★ 本表是 Domain 这一维存在的理由，也是外壳真正要写的那个 switch 的样子。
//
// 只看 Kind 的话，第 1、2 行都是「数据损坏」，第 3、4 行都是「用法错误」——但每一对的处置
// 都相反：提示更新客户端 vs 清缓存重下；重新登录 vs 改模块配置。少了 Domain，外壳拿到 Kind
// 会以为信息够了，实际做不出正确动作。
func TestDomainSeparatesOtherwiseIdenticalKinds(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantDomain app.ErrorDomain
		wantKind   app.ErrorKind
		shellDoes  string
	}{
		{"游戏响应解不开", &app.APIError{Message: "x"}, app.ErrorFromGameAPI, app.ErrorRejected,
			"把服务器给的理由呈现给用户"},
		{"会话被顶掉", &app.SessionBreakError{Cause: errors.New("顶号")},
			app.ErrorFromGameAPI, app.ErrorTransient, "提示复查后重跑"},
		{"没注入验证码求解器", app.ErrNoSolver, app.ErrorFromCredential, app.ErrorMisuse,
			"补上求解器，或提示用户改用别的登录方式"},
		{"模块要母数据但没启用", app.ErrMasterdataUnavailable, app.ErrorFromAutomation, app.ErrorMisuse,
			"带上 withMasterdata 重跑一次"},
		{"未登录就 Run", app.ErrNotLoggedIn, app.ErrorFromApp, app.ErrorMisuse,
			"先调 Login"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := app.ClassifyError(c.err)
			if got.Domain != c.wantDomain {
				t.Errorf("Domain = %v, want %v（外壳据此决定：%s）", got.Domain, c.wantDomain, c.shellDoes)
			}
			if got.Kind != c.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, c.wantKind)
			}
		})
	}

	// 同为 ErrorMisuse 的三条，来源各不相同——这正是「凭据不对」「配置不对」「调用顺序不对」
	// 三种完全不同的引导得以分开的依据。
	misuse := []error{app.ErrNoSolver, app.ErrMasterdataUnavailable, app.ErrNotLoggedIn}
	seen := map[app.ErrorDomain]bool{}
	for _, err := range misuse {
		c := app.ClassifyError(err)
		if c.Kind != app.ErrorMisuse {
			t.Fatalf("前提失效：%v 本应是 ErrorMisuse", err)
		}
		if seen[c.Domain] {
			t.Errorf("%v 的来源域与前面某条重复，三种引导会混作一谈", err)
		}
		seen[c.Domain] = true
	}
}
