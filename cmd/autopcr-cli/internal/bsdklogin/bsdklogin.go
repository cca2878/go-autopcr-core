// Package bsdklogin 用 bilibili 账密登录取得 (uid, access_key)。
//
// 它补齐 accesskey「四要素直传」跳过的那一段【冷启动】：账密 → bilibili external v3 登录
// → (uid, access_key)。登录（含 bilibili 账号侧的 geetest 验证码）全部由独立模块
// github.com/cca2878/bsdkv3-go 完成；本包只把拿到的 (uid, access_key) 返回，由调用方
// （app.Session.Login）据此装配游戏侧「四要素直传」凭据——本包不构造 Credential。
//
// 注：此路径仅适用于 bilibili 官服（ChannelBSDK）；渠道服（qsdk）走不同的登录 SDK。
//
// 归属：本包与同级 remote 求解器是【外壳(imperative shell)侧】组件——账密→access_key
// 只是冷启动手段，与无头客户端核心解耦，故不放核心树 internal/client 下，随 CLI 这个外壳
// 就近安放（见架构决策：SDK 移出核心）。
//
// 验证码求解统一：bilibili 登录与游戏服风控（is_risk）虽是两套独立挑战，但都经 gtrv
// 远程求解服务。集成方构造【一个】gtrv.Validator，经 WithValidator 注入登录、经游戏侧
// captcha.Solver 注入风控（见同级 remote 包），即两端复用同一求解器。不注入时 bsdkv3-go
// 用其内置 gtrv 求解器（开箱即用）。
package bsdklogin

import (
	"context"
	"fmt"
	"net/http"

	bsdkv3 "github.com/cca2878/bsdkv3-go"
	"github.com/cca2878/gtrv-go"

	"github.com/cca2878/go-autopcr/internal/client/credential/accesskey"
)

// options 定制登录行为。
type options struct {
	appKey    string
	transport *http.Transport
	validator gtrv.Validator
}

// Option 定制 bsdk 登录。
type Option func(*options)

// WithAppKey 覆盖 bilibili SDK AppKey（默认 PCR 国服 bsdkv3.AppkeyPcr）。
func WithAppKey(appKey string) Option {
	return func(o *options) {
		if appKey != "" {
			o.appKey = appKey
		}
	}
}

// WithTransport 注入共享的底层 *http.Transport。bsdk 登录网关与其验证码求解客户端
// 都会复用它，从而与无头客户端的游戏 API / 资源下载统一 proxy/TLS/连接池（只配一次）。
//
// 注意：登录服与 geetest 求解服务走默认 HTTP（含 h2），故这里应传常规 transport，
// 不要传游戏服专用的「强制 HTTP/1.1」那一份。
func WithTransport(rt *http.Transport) Option {
	return func(o *options) { o.transport = rt }
}

// WithValidator 注入 bilibili 登录用的验证码求解器（gtrv.Validator）。集成方若在别处也需
// 同一求解能力（如游戏服风控），构造【一个】gtrv.Validator 同时传入此处与游戏侧 Solver，
// 即可两端复用同一求解器。不设时 bsdkv3-go 用其内置 gtrv 远程求解器（开箱即用）。
func WithValidator(v gtrv.Validator) Option {
	return func(o *options) { o.validator = v }
}

// Login 用 bilibili 账密完成 external v3 登录，取得并返回 (uid, access_key)。整个 bilibili
// 登录序列（bootstrap→cipher→login→验证码）由 bsdkv3-go 完成；失败时原样透出其结构化错误
// （可用 errors.Is 命中 bsdkv3.ErrLogin 等）。得到的 (uid, access_key) 即游戏侧「四要素直传」
// 所需——交给 app.Session.Login 即可。
func Login(ctx context.Context, username, password string, opts ...Option) (uid, accessKey string, err error) {
	o := options{appKey: bsdkv3.AppkeyPcr}
	for _, opt := range opts {
		opt(&o)
	}
	if username == "" || password == "" {
		return "", "", fmt.Errorf("bsdk 登录：用户名与密码均不能为空")
	}

	// bsdkv3 的配置类型未导出，无法建 []Option 切片；好在 NewClient 变参且 nil 值无害
	// （内部 nil 检查会回落到默认），故未设置项直接以 nil 透传。
	cli, err := bsdkv3.NewClient(ctx, o.appKey,
		bsdkv3.WithClientTransport(o.transport),
		bsdkv3.WithClientValidator(o.validator),
	)
	if err != nil {
		return "", "", fmt.Errorf("bsdk 初始化失败: %w", err)
	}

	acc, err := cli.Auth.Login(ctx, bsdkv3.UserInfo{
		Username: username,
		Password: password,
		Channel:  accesskey.ChannelBSDK,
	})
	if err != nil {
		return "", "", fmt.Errorf("bsdk 登录失败: %w", err)
	}

	return acc.Uid, acc.AccessKey, nil
}
