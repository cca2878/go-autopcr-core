// Package remote 提供走 gtrv 远程 geetest 求解服务的验证码求解器。
//
// 它把「一个」gtrv.Validator 同时供给两处需求：既实现游戏服风控（is_risk）所需的
// captcha.Solver，又经 Validator() 暴露底层 gtrv.Validator 供 bilibili 登录注入
// （bsdklogin.WithValidator → bsdkv3.WithClientValidator）。一处构造、两端复用，
// gtrv 参数（求解服务地址/UA/轮询）只配一次。
//
// gtrv 求解服务自管整个 geetest 会话、直接吐 token（Validate 不吃目标服务器的 challenge），
// 故同一实例天然可同时服务 bilibili 登录与游戏风控两套挑战。
package remote

import (
	"context"

	"github.com/cca2878/gtrv-go"

	"github.com/cca2878/go-autopcr/internal/client/credential/captcha"
)

// Solver 用单个 gtrv.Validator 求解验证码，实现 captcha.Solver。
type Solver struct {
	v gtrv.Validator
}

// 确保实现游戏侧验证码求解端口。
var _ captcha.Solver = (*Solver)(nil)

// New 构造 gtrv 远程求解器。opts 透传 gtrv 选项（求解服务地址/UA/轮询参数等）；不传则用
// gtrv 默认（pcrd.tencentbot.top）。底层 HTTP 用 gtrv 默认客户端，超时由传入 Solve 的 ctx 控制。
func New(opts ...gtrv.Option) *Solver {
	return &Solver{v: gtrv.NewRemoteValidator(nil, opts...)}
}

// Wrap 用已构造好的 gtrv.Validator 建求解器（需自定义 HTTP 客户端/代理/降级链时）。
func Wrap(v gtrv.Validator) *Solver { return &Solver{v: v} }

// Validator 返回底层 gtrv.Validator，供注入 bilibili 登录（bsdklogin.WithValidator）。
func (s *Solver) Validator() gtrv.Validator { return s.v }

// Solve 实现 captcha.Solver：求解并映射为四要素（seccode = validate|jordan，复刻原项目约定）。
func (s *Solver) Solve(ctx context.Context) (*captcha.Result, error) {
	r, err := s.v.Validate(ctx)
	if err != nil {
		return nil, err
	}
	return &captcha.Result{
		Challenge: r.Challenge,
		Validate:  r.Validate,
		Seccode:   r.Validate + "|jordan",
	}, nil
}
