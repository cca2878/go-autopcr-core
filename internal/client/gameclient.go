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

	"github.com/cca2878/go-autopcr/internal/client/credential"
	"github.com/cca2878/go-autopcr/internal/client/gameapi"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr/internal/client/internal/session"
	"github.com/cca2878/go-autopcr/internal/client/internal/transport"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
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
	tr := transport.New(cred, transport.WithHTTPClient(gameHTTP), transport.WithLogger(o.logger))

	state := gamestate.New()
	registry := gamestate.DefaultRegistry()
	// 中间件链（外→内）：错误处理 → 状态折叠 → 传输。
	tr.Use(
		transport.ErrorHandler(transport.DefaultRetries),
		foldingMiddleware(state, registry),
	)

	return &client{
		GameAPI:    gameapi.New(tr),
		cred:       cred,
		tr:         tr,
		state:      state,
		rt:         rt,
		mdEnabled:  o.mdEnabled,
		mdCacheDir: o.mdCacheDir,
		logger:     o.logger,
	}
}

func (g *client) Login(ctx context.Context) error {
	if err := session.Login(ctx, g.tr, g.cred); err != nil {
		return err
	}
	if g.mdEnabled {
		return g.ensureMasterdata(ctx)
	}
	return nil
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
	q, err := r.Ensure(ctx, g.state.ManifestVer, g.state.ResURL)
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
