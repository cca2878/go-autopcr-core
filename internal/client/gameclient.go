// Package client 组装「无头游戏客户端」——本项目的枢纽缝（S2）。
//
// 它把 L2 传输/会话、L3 母数据/状态 装配成一个可被上层「操作」的对象：登录后
// 通过 GameClient 接口读取玩家状态（M2），后续里程碑再在其上叠加高层动作方法，
// 供自动化模块（M3）调用。上层只依赖 GameClient 接口，可用 mock 独立测试。
//
// 客户端拥有一个共享的底层 *http.Transport：游戏 API 与 masterdata 资源 CDN 下载
// 复用它（统一 proxy/TLS/HTTP1.1/连接池，只配一次），各自再包成不同超时的 http.Client。
package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/cca2878/go-autopcr-core/internal/client/credential"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/appversion"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/session"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
)

// GameClient 是无头客户端对上层暴露的接口（S2 缝）。
type GameClient interface {
	// Login 执行登录序列并把玩家状态折叠进 Data()；若启用了 masterdata，登录后按
	// 下发的 res + manifest_ver 确保干净库就绪并打开只读查询句柄。
	Login(ctx context.Context) error
	// Data 返回聚合的玩家状态。
	Data() *gamestate.PlayerState
	// Masterdata 返回母数据只读查询面；未启用或登录前为 nil。
	// 返回接口（而非具体 *Query）以便上层 mock 单测。
	Masterdata() masterdata.Reader
	// ServerTime 返回最近同步到的服务器时间（Unix 秒）。
	ServerTime() int64
	// Close 释放客户端持有的资源（母数据库连接）。
	Close() error

	// GameAPI 是游戏 API 能力面（独立公开子包 client/gameapi，供自动化模块按参数调用）。
	// 随功能增长在该子包里按域累加，与上面的生命周期方法分开组织。
	gameapi.GameAPI
}

// Option 定制无头客户端。
type Option func(*options)

type options struct {
	logger      *slog.Logger
	proxy       *url.URL
	insecureTLS bool
	mdEnabled   bool
	mdCacheDir  string
}

// WithLogger 设置日志器。
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.logger = l } }

// WithProxy 让所有 HTTP（游戏 API + 资源下载）经由指定代理（调试抓包用）。
func WithProxy(u *url.URL) Option { return func(o *options) { o.proxy = u } }

// WithInsecureTLS 对所有 HTTP 跳过证书校验（调试用；勿用于生产）。
func WithInsecureTLS() Option { return func(o *options) { o.insecureTLS = true } }

// WithMasterdata 启用 masterdata：登录后按下发 res + manifest_ver 确保干净库并打开查询句柄。
// cacheDir 下按版本缓存干净库。反混淆表（rainbow）已内嵌于客户端。
func WithMasterdata(cacheDir string) Option {
	return func(o *options) { o.mdEnabled = true; o.mdCacheDir = cacheDir }
}

type client struct {
	gameapi.GameAPI // 组合游戏 API 能力面访问器（gc.Account()/gc.Daily()…，见 client/gameapi）

	cred  credential.Credential
	tr    *transport.Client
	state *gamestate.PlayerState
	rt    *http.Transport // 共享底层传输：游戏 API 与资源 CDN 下载共用
	md    *masterdata.Query
	guard *sessionGuard // 严重错误码 → 重走登录序列（见 relogin.go）

	// callMu 串行化「一次调用的全过程」：网络重试 + 状态折叠 + 传输。传输层自己那把锁只
	// 盖住 HTTP 那一段，折叠在它外面，两个并发请求会同时改 PlayerState 的 map/slice。
	callMu sync.Mutex

	// masterdata 装配所需（登录后用下发 res 构建源与 Manager）。
	mdEnabled  bool
	mdCacheDir string
	logger     *slog.Logger
}

// New 装配一个无头客户端。默认不接 masterdata（如仅做连通性探针）；用 WithMasterdata
// 启用后，登录后自动按下发 res + manifest_ver 确保干净库并打开查询句柄。
func New(cred credential.Credential, opts ...Option) GameClient {
	o := &options{logger: slog.Default()}
	for _, opt := range opts {
		opt(o)
	}

	rt := transport.NewRoundTripper(o.proxy, o.insecureTLS)
	gameHTTP := &http.Client{Timeout: transport.DefaultTimeout, Transport: rt}
	trOpts := []transport.Option{transport.WithHTTPClient(gameHTTP), transport.WithLogger(o.logger)}
	// APP-VER 版本自愈落盘（见 appversion 包）：复用 masterdata 缓存目录，未启用 masterdata
	// 时 o.mdCacheDir 为空、appversion 整包空操作，退化为每个 Client 生命周期内自愈一次。
	// RES-VER 没有自愈回调（它不靠拒绝重试，见 session.Login），落盘发生在 Login 成功之后。
	if o.mdCacheDir != "" {
		trOpts = append(trOpts, transport.WithOnVersionUpdate(func(v string) {
			appversion.Write(o.mdCacheDir, "APP-VER", v)
		}))
	}
	tr := transport.New(cred, trOpts...)
	if v, ok := appversion.Read(o.mdCacheDir, "APP-VER"); ok {
		tr.SetHeader("APP-VER", v)
	}
	if v, ok := appversion.Read(o.mdCacheDir, "RES-VER"); ok {
		tr.SetHeader("RES-VER", v)
	}

	state := gamestate.New()
	registry := gamestate.DefaultRegistry()

	g := &client{
		GameAPI:    gameapi.New(tr),
		cred:       cred,
		tr:         tr,
		state:      state,
		rt:         rt,
		mdEnabled:  o.mdEnabled,
		mdCacheDir: o.mdCacheDir,
		logger:     o.logger,
	}
	g.guard = &sessionGuard{login: g.relogin, expired: g.sessionExpired, logger: o.logger}

	// 中间件链（外→内）：会话重登 → 串行化 → 错误处理 → 状态折叠 → 传输。
	// 重登在最外层：网络重试应先耗尽，且重登发出的登录请求要经过折叠中间件才能更新状态。
	tr.Use(
		g.guard.middleware(),
		serializeMiddleware(&g.callMu),
		transport.ErrorHandler(transport.DefaultRetries),
		foldingMiddleware(state, registry),
	)
	return g
}

func (g *client) Login(ctx context.Context) error {
	if err := session.Login(markRelogin(ctx), g.tr, g.cred, g.logger); err != nil {
		return err
	}
	g.guard.markFresh()
	appversion.Write(g.mdCacheDir, "RES-VER", g.tr.Header("RES-VER"))
	if g.mdEnabled {
		return g.ensureMasterdata(ctx)
	}
	return nil
}

// relogin 是会话失效时的自愈动作：用【同一凭据】重跑登录序列（重新获取 access_key 是
// 外壳的事，核心不碰）。不重建母数据——会话失效与母数据版本无关，且查询句柄可能正被
// 模块持有，中途换掉它比留着更危险；真的版本变更会走维护/版本升级路径。
func (g *client) relogin(ctx context.Context) error {
	return session.Login(ctx, g.tr, g.cred, g.logger)
}

// sessionExpired 报告会话是否已越过每日重置点（load/index 下发的 daily_reset_time）。
// 服务端到点即丢弃会话，故守卫在发包前据此主动重登，而不是等下一个请求撞上会话错误。
// 尚未登录（或服务端未下发该字段）时恒为 false。
func (g *client) sessionExpired() bool {
	exp := g.state.DailyResetTime
	return exp > 0 && g.tr.ServerTime() >= exp
}

// ensureMasterdata 用登录折叠得到的 manifest_ver + res 确保干净库就绪并打开查询句柄。
//
// 确保管线（下载→反混淆→缓存→只读打开）由 masterdata.Refresher 统一提供（与免凭证刷新
// 路径共用）；资源下载复用客户端的共享 Transport（与游戏 API 同一 proxy/TLS/连接池）。
func (g *client) ensureMasterdata(ctx context.Context) error {
	r, err := NewMasterdataRefresher(g.mdCacheDir,
		masterdata.WithTransport(g.rt),
		masterdata.WithLogger(g.logger),
	)
	if err != nil {
		return err
	}
	q, err := r.Ensure(ctx, g.state.ManifestVer, g.state.ResURLs)
	if err != nil {
		return err
	}
	g.md = q
	return nil
}

func (g *client) Data() *gamestate.PlayerState { return g.state }

func (g *client) Masterdata() masterdata.Reader {
	if g.md == nil {
		return nil // 避免把 nil *Query 包成非 nil 接口（typed-nil 陷阱）
	}
	return g.md
}

func (g *client) ServerTime() int64 { return g.tr.ServerTime() }

func (g *client) Close() error { return g.md.Close() }

// serializeMiddleware 让「重试 + 折叠 + 传输」整体互斥（对应原项目把 mutexhandler 注册在
// 最外层的做法——它那把锁同样盖住 datamgr 的折叠）。
//
// 位置很关键：它必须在重登守卫【之内】。守卫的 ensure 会在本层加锁【之前】跑完整套登录
// 序列，那些请求自身也走这条链；若把锁放到守卫之外，重登就会在同一把非重入锁上自死锁。
func serializeMiddleware(mu *sync.Mutex) transport.Middleware {
	return func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			mu.Lock()
			defer mu.Unlock()
			return next(ctx, req, out)
		}
	}
}

// foldingMiddleware 在每次成功响应后，把响应折叠进玩家状态
// （对应原项目 datamgr 作为管道组件拦截响应的做法）。
func foldingMiddleware(s *gamestate.PlayerState, registry *gamestate.Registry) transport.Middleware {
	return func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			header, err := next(ctx, req, out)
			if err == nil && out != nil {
				registry.Apply(s, out)
			}
			return header, err
		}
	}
}
