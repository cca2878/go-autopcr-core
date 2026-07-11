package masterdata

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/accesskey"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/discovery"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/urlx"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/asset"
)

// assetDownloadTimeout 是母数据 CDN 下载的 http 超时（母数据包达几十 MB，给足余量）。
const assetDownloadTimeout = 5 * time.Minute

// Refresher 装配母数据「确保就绪」管线，并支持【免凭证刷新】。
//
// 两条路径共用同一确保内核（下载→反混淆→缓存→只读打开）：
//   - Ensure：已知 manifest_ver + res（登录已学到）时直接确保——供无头客户端登录后调用。
//   - Refresh：不登录、无凭证，自行握手（source_ini + maintenance，见 discovery）取最新
//     manifest_ver + res 再确保——保证程序总能拿到并展示最新数据，无需已配置凭证与登录。
type Refresher struct {
	cacheDir string
	rainbow  Rainbow
	rt       *http.Transport
	channel  string
	logger   *slog.Logger
}

// RefresherOption 定制 Refresher。
type RefresherOption func(*Refresher)

// WithTransport 复用一个共享底层 *http.Transport（与游戏 API / 资源下载统一 proxy/TLS/连接池）。
func WithTransport(rt *http.Transport) RefresherOption { return func(r *Refresher) { r.rt = rt } }

// WithChannel 设置免凭证握手用的渠道（决定 APIRoot / 静态请求头）。默认官服 bsdk。
func WithChannel(ch string) RefresherOption { return func(r *Refresher) { r.channel = ch } }

// WithLogger 设置日志器。
func WithLogger(l *slog.Logger) RefresherOption { return func(r *Refresher) { r.logger = l } }

// NewRefresher 构造 Refresher。cacheDir 下按版本缓存干净库；rainbow 为反混淆表。
func NewRefresher(cacheDir string, rainbow Rainbow, opts ...RefresherOption) *Refresher {
	r := &Refresher{
		cacheDir: cacheDir,
		rainbow:  rainbow,
		channel:  accesskey.ChannelBSDK,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.rt == nil {
		r.rt = transport.NewRoundTripper(nil, false)
	}
	return r
}

// Ensure 用已知 manifest_ver + res 确保干净库就绪并打开只读查询句柄。
//
// res 为 nil（下发缺失）时回退内置默认 CDN。资源下载复用 Refresher 的共享 Transport。
func (r *Refresher) Ensure(ctx context.Context, manifestVer string, res *url.URL) (*Query, error) {
	ver, err := strconv.Atoi(manifestVer)
	if err != nil {
		return nil, fmt.Errorf("manifest_ver %q 非法: %w", manifestVer, err)
	}
	if res == nil {
		res = urlx.MustParseBase(asset.DefaultRes)
		r.logger.Warn("res 为空，回退内置默认 CDN", "res", res)
	}
	src := asset.NewSource(
		asset.WithRes(res),
		asset.WithHTTPClient(&http.Client{Timeout: assetDownloadTimeout, Transport: r.rt}),
	)
	mgr := NewManager(r.cacheDir, r.rainbow, src)
	path, err := mgr.EnsureDB(ctx, ver)
	if err != nil {
		return nil, err
	}
	return Open(path)
}

// Refresh 免凭证刷新：不登录即自取最新 manifest_ver + res，确保干净库并打开只读查询句柄。
func (r *Refresher) Refresh(ctx context.Context) (*Query, error) {
	cred, err := accesskey.Anonymous(r.channel)
	if err != nil {
		return nil, err
	}
	tc := transport.New(cred,
		transport.WithHTTPClient(&http.Client{Timeout: transport.DefaultTimeout, Transport: r.rt}),
		transport.WithLogger(r.logger),
	)
	disc, err := discovery.Discover(ctx, tc)
	if err != nil {
		return nil, err
	}
	return r.Ensure(ctx, disc.ManifestVer, disc.ResURL)
}
