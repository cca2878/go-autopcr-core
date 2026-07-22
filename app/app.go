// Package app 是 go-autopcr 的【应用服务门面】——把无头客户端(S2)与自动化运行器(S3)
// 装配成产品级操作（登录 / 列模块 / 跑任务 / 读玩家态与母数据），供所有前端共享：
// CLI、未来的 web server、以及经 gomobile 的移动端 wrapper（go-autopcr-mobile）。
//
// 定位（functional core / imperative shell）：app 是地道 Go（带 context / 结构体 / error），
// 不掺任何 gomobile 味道；有状态的会话隔离在 Session 里，其上的运行是无状态、data-in/data-out。
//
//   - 凭据只吃 (channel, uid, access_key)：账密→access_key 的冷启动属外壳职责（见 go-autopcr
//     外壳侧的 bsdk 组件），拿到后交给 Login 即可。
//   - 目录一律由调用方传入（见 Dirs）：核心不假设工作目录，桌面传本地路径、安卓传 filesDir。
//   - 验证码求解器由外壳经 WithCaptchaSolver 注入；不注入则触发风控(is_risk)时硬失败。
//
// 详见架构决策：SDK/captcha 移出核心、核心保持确定性内核。
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules"
	"github.com/cca2878/go-autopcr-core/internal/client"
	"github.com/cca2878/go-autopcr-core/internal/client/credential/accesskey"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
)

// Dirs 是外壳提供给核心的文件系统位置。核心不假设任何工作目录——一切目录由调用方【显式传入】
// （桌面传本地路径，安卓传 filesDir/cacheDir 等应用私有目录）。沿用 gtlv 的经验：host 负责
// 决定并传入位置、核心只使用。
//
// 目前核心仅需 Cache（rainbow 已内嵌、验证码模型属外壳侧求解器）；将来核心新增需落盘的资产，
// 在此加字段即可，不改 NewSession 签名。
type Dirs struct {
	// Cache 是派生产物（按版本缓存的母数据 SQLite）的存放目录。
	Cache string
}

// Session 是一次游戏会话：登录一次、可多次跑模块。它持有装配好的无头客户端（会话/玩家态/
// 母数据句柄），故【有状态、非并发安全】；用毕 Close。
//
// 生命周期归调用方（外壳）：建一个 Session 长期复用，避免每次操作重建（登录 + 母数据 ensure
// 是昂贵的一次性成本）。
type Session struct {
	dirs     Dirs
	registry *automation.Registry

	// 构造期选项。
	logger      *slog.Logger
	proxy       *url.URL
	insecureTLS bool
	solver      Solver
	collector   automation.Collector // 遥测采集端口；nil＝不采集（模块 Emit 为 no-op）

	gc client.GameClient // 登录前为 nil
}

// Option 定制 Session。
type Option func(*Session)

// WithLogger 设置日志器（默认 slog.Default()）。
func WithLogger(l *slog.Logger) Option { return func(s *Session) { s.logger = l } }

// WithProxy 让游戏 API 与母数据下载经指定代理（调试抓包）。
func WithProxy(u *url.URL) Option { return func(s *Session) { s.proxy = u } }

// WithInsecureTLS 跳过 HTTPS 证书校验（调试用；勿用于生产）。
func WithInsecureTLS() Option { return func(s *Session) { s.insecureTLS = true } }

// WithCaptchaSolver 注入验证码求解器（外壳侧构造，如 gtrv 远程或本地 wasm）。不注入则
// 触发风控(is_risk)时以 captcha.ErrNoSolver 硬失败——正常登录不需要它。
func WithCaptchaSolver(s Solver) Option { return func(sess *Session) { sess.solver = s } }

// WithCollector 注入遥测采集端口（外壳侧构造：缓冲/持久/上传）。不注入则模块的 rc.Emit 为
// no-op——采集与否不影响模块业务判定与 Run 结果。
func WithCollector(c Collector) Option { return func(sess *Session) { sess.collector = c } }

// NewSession 创建会话。dirs 提供核心所需的文件系统位置（见 Dirs）。
func NewSession(dirs Dirs, opts ...Option) *Session {
	s := &Session{
		dirs:     dirs,
		registry: modules.DefaultRegistry(),
		logger:   slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Registry 暴露模块注册表，供前端列出/挑选模块与预设。
func (s *Session) Registry() *Registry { return s.registry }

// Login 用「四要素直传」凭据登录：channel 取 ChannelBSDK / ChannelQSDK。withMasterdata 为真时，
// 登录后按下发 res + manifest_ver 确保干净母数据库并打开只读查询面（落于 dirs.Cache）。
//
// 账密→(uid, access_key) 的冷启动不在此——由外壳先行完成，再把 (uid, access_key) 交进来。
func (s *Session) Login(ctx context.Context, channel, uid, accessKey string, withMasterdata bool) error {
	var akOpts []accesskey.Option
	if s.solver != nil {
		akOpts = append(akOpts, accesskey.WithCaptchaSolver(s.solver))
	}
	cred, err := accesskey.New(channel, uid, accessKey, akOpts...)
	if err != nil {
		return err
	}

	opts := []client.Option{client.WithLogger(s.logger)}
	if s.proxy != nil {
		opts = append(opts, client.WithProxy(s.proxy))
	}
	if s.insecureTLS {
		opts = append(opts, client.WithInsecureTLS())
	}
	if withMasterdata {
		opts = append(opts, client.WithMasterdata(s.dirs.Cache))
	}

	gc := client.New(cred, opts...)
	if err := gc.Login(ctx); err != nil {
		_ = gc.Close()
		return err
	}
	// 重复 Login（如换号或凭据轮换）须先释放上一个客户端，否则其母数据库连接会一直泄漏。
	if s.gc != nil {
		_ = s.gc.Close()
	}
	s.gc = gc
	return nil
}

// Player 返回登录后聚合的玩家状态；未登录时为 nil。
func (s *Session) Player() *PlayerState {
	if s.gc == nil {
		return nil
	}
	return s.gc.Data()
}

// Masterdata 返回母数据只读查询面；未启用（或未登录）时为 nil。
func (s *Session) Masterdata() Reader {
	if s.gc == nil {
		return nil
	}
	return s.gc.Masterdata()
}

// ServerTime 返回最近同步到的服务器时间（Unix 秒）；未登录时为 0。
func (s *Session) ServerTime() int64 {
	if s.gc == nil {
		return 0
	}
	return s.gc.ServerTime()
}

// Run 依次执行 tasks，返回每个任务的结果与终止因由（须先 Login 成功）。单任务=长度 1、批处理=
// 多元素，统一走 automation.Run；单个任务失败/跳过不影响其余。
//
// obs 是可选的进度端口（见 Observer）：非 nil 时按任务边界推送进度事件，nil 即无进度。返回的
// error 非 nil 表示【被取消】（ctx 取消/超时），此时 []Result 只含已完成任务；nil 表示全部跑完。
func (s *Session) Run(ctx context.Context, tasks []Task, obs Observer) ([]Result, error) {
	if s.gc == nil {
		return nil, errors.New("会话未登录：请先 Login")
	}
	return automation.Run(ctx, s.gc, s.registry, tasks, obs, s.collector)
}

// Close 释放会话资源（母数据库连接）。未登录时安全返回 nil。
func (s *Session) Close() error {
	if s.gc == nil {
		return nil
	}
	return s.gc.Close()
}

// DefaultRegistry 返回内置模块的默认注册表（含全部模块与预设），供【无需会话】地列出/挑选
// 模块（如 CLI 的 --list）。会话内的 Run 用其自有注册表按名解析任务，二者内容一致。
func DefaultRegistry() *Registry { return modules.DefaultRegistry() }

// RefreshMasterdata 【免登录/免凭证】确保并打开最新母数据：自行握手取最新版本、必要时下载
// 反混淆落库（落于 dirs.Cache），返回只读查询句柄（调用方负责 Close）。channel 取
// ChannelBSDK / ChannelQSDK；logger 为 nil 时用 slog.Default()。
func RefreshMasterdata(ctx context.Context, dirs Dirs, channel string, logger *slog.Logger) (MasterdataHandle, error) {
	if logger == nil {
		logger = slog.Default()
	}
	r, err := client.NewMasterdataRefresher(dirs.Cache,
		masterdata.WithChannel(channel),
		masterdata.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	q, err := r.Refresh(ctx)
	if err != nil {
		// 显式返回 nil 接口：避免把 (*masterdata.Query)(nil) 包成一个非 nil 的
		// MasterdataHandle（typed-nil 陷阱），让调用方的 handle != nil 判断可靠。
		return nil, err
	}
	return q, nil
}
