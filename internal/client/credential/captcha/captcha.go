// Package captcha 定义验证码求解'端口'——只定义端口，不带任何实现。
//
// 架构决策（captcha 移出核心）：核心自身不携带任何验证码求解器实现、不依赖任何远程/本地
// 求解库（gtrv / wasm 等）。具体求解器由外壳(imperative shell)构造并经
// accesskey.WithCaptchaSolver 注入。未注入求解器时，触发风控(is_risk)的登录会以
// ErrNoSolver 硬失败——响亮、可诊断，而非静默或占位放行。
package captcha

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// ErrNoSolver 表示凭据未配置验证码求解器。触发风控(is_risk)时若无求解器，登录据此
// 硬失败：这是有意的——核心不携带求解器实现，求解能力一律由外壳注入。外壳可用
// errors.Is 命中它，据此提示用户 / 注入求解器 / 采集数据。
//
// 归 KindMisuse 而非 KindRejected：风控本身是对端行为，但"没人能解它"是外壳漏了装配，
// 补上求解器即可，与账号真被风控拦下是两回事。
var ErrNoSolver = errs.DomainCredential.New(errs.KindMisuse, "captcha: 未配置验证码求解器")

// Result 是一次 gt 求解的结果。
type Result struct {
	Challenge string
	Validate  string
	Seccode   string
}

// Solver 是验证码求解端口。核心只定义此端口；具体实现（gtrv 远程 / 本地 wasm 等）
// 由外壳构造后经 accesskey.WithCaptchaSolver 注入。
type Solver interface {
	Solve(ctx context.Context) (*Result, error)
}
