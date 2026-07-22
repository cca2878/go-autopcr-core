package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/credential"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/urlx"
)

// DefaultTimeout 是单次请求的默认超时（对应原项目 timeout=10）。
const DefaultTimeout = 10 * time.Second

// Client 是面向单个账号的传输客户端：封装加密、编解码、HTTP 与会话头维护。
//
// 一个 Client 对应一个逻辑会话，内部以互斥锁串行化所有请求（复刻原
// apiclient 的 per-client Lock），因此其可变状态无需额外并发保护。
type Client struct {
	cred   credential.Credential
	http   *http.Client
	logger *slog.Logger

	mu         sync.Mutex
	headers    map[string]string
	servers    []*url.URL
	active     int
	viewerID   int64
	serverTime int64
	localTime  time.Time

	handler Handler
}

// Option 用于定制 Client。
type Option func(*Client)

// WithHTTPClient 覆盖默认 http.Client。生产装配一律经此传入共享 Transport（见 NewRoundTripper），
// 代理/证书等调试开关在构建那个 Transport 时决定，故此处不再重复提供。
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithLogger 设置日志器。
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// New 构造一个传输客户端。初始服务器取凭据的 APIRoot；登录序列会用真实列表覆盖它。
func New(cred credential.Credential, opts ...Option) *Client {
	c := &Client{
		cred:    cred,
		logger:  slog.Default(),
		headers: cred.Header(),
		servers: []*url.URL{urlx.MustParseBase(cred.APIRoot())},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: DefaultTimeout, Transport: NewRoundTripper(nil, false)}
	}
	c.handler = c.transport
	return c
}

// NewRoundTripper 返回强制 HTTP/1.1 的传输层，并应用代理 / 证书调试选项。
//
// 官方客户端走 HTTP/1.1，且在 TLS 的 ALPN 中显式声明 "http/1.1"。这里必须做两件事：
//  1. 禁用客户端 h2（ForceAttemptHTTP2=false + 空 TLSNextProto），使响应按 HTTP/1.1 解析；
//  2. 显式设置 TLSClientConfig.NextProtos = ["http/1.1"]。
//
// 缺第 2 步时 Go 不会在 ALPN 中声明任何协议，服务器遂自行选择默认协议——部分游戏
// 服务器（如 le1-*）在空 ALPN 下默认回落 HTTP/2，其 h2 帧会被误判为畸形 HTTP/1 响应。
//
// 导出以便上层在「游戏 API」与「资源 CDN 下载」之间共享同一底层 Transport（统一
// 代理/TLS/连接池，只配一次），各自再包成不同超时的 *http.Client。
func NewRoundTripper(proxy *url.URL, insecure bool) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ForceAttemptHTTP2 = false
	tr.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{} //nolint:gosec // InsecureSkipVerify 按需在下方设置
	}
	tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
	if proxy != nil {
		tr.Proxy = http.ProxyURL(proxy)
	}
	if insecure {
		tr.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // 调试选项，显式启用
	}
	return tr
}

// Use 用给定中间件包裹传输处理器（靠前的中间件在外层）。
func (c *Client) Use(mws ...Middleware) {
	c.handler = Chain(mws...)(c.transport)
}

// Handler 返回组装后的请求处理器。
func (c *Client) Handler() Handler { return c.handler }

// ServerTime 返回当前的服务器时间（Unix 秒）＝最近一次同步值 + 其后本地流逝的时间
// （对应 ref apiclient.time 的 time.time() - _local_time + _server_time）。
//
// 必须加上流逝量：登录后可能先下几分钟母数据再跑模块，长驻会话更可能空闲数小时；直接返回
// 同步时刻的旧值会让时间门禁模块（赛马/公主祭/剧情窗口…）在边界附近判错开放状态。
func (c *Client) ServerTime() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.localTime.IsZero() {
		return c.serverTime
	}
	return c.serverTime + int64(time.Since(c.localTime).Seconds())
}

// SetServers 覆盖服务器列表（base URL）并复位当前索引。
func (c *Client) SetServers(servers []*url.URL) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(servers) > 0 {
		c.servers = servers
		c.active = 0
	}
}

// SetHeader 设置一个请求头。
func (c *Client) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers[key] = value
}

// Call 是泛型门面：分配 *R、走处理器链、返回解码后的响应。
func Call[R any](ctx context.Context, c *Client, req protocol.Request) (*R, error) {
	out := new(R)
	if _, err := c.handler(ctx, req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// transport 是最内层处理器：加密/编码/HTTP/解码/维护会话头。
func (c *Client) transport(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, err := genKey()
	if err != nil {
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}
	crypted := req.Crypted()

	// 注入 viewer_id
	vid := strconv.FormatInt(c.viewerID, 10)
	if crypted {
		enc, err := encryptViewerID(vid, key)
		if err != nil {
			return protocol.ResponseHeader{}, gameerr.Network(err)
		}
		req.SetViewerID(enc)
	} else {
		req.SetViewerID(vid)
	}

	// 编码请求体
	var body []byte
	if crypted {
		body, err = packCrypted(req, key)
	} else {
		body, err = json.Marshal(req)
	}
	if err != nil {
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}

	full := c.servers[c.active].ResolveReference(req.URL())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, full.String(), bytes.NewReader(body))
	if err != nil {
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}
	// 直接写 map（而非 Header.Set）以保留精确大小写；游戏服务器对头名大小写敏感。
	for k, v := range c.headers {
		httpReq.Header[k] = []string{v}
	}

	// 逐请求耗时观测：仅在 debug 级启用时计时/记录。非 debug 模式下走这一个布尔判断即
	// 短路，连 time.Now()、参数装箱都不发生——零输出、零开销。
	debugTiming := c.logger.Enabled(ctx, slog.LevelDebug)
	var start time.Time
	if debugTiming {
		c.logger.Debug("game request", "url", req.URL(), "crypted", crypted, "server", c.servers[c.active])
		start = time.Now()
	}
	resp, err := c.http.Do(httpReq)
	if debugTiming {
		c.logger.Debug("game response", "url", req.URL(), "elapsed", time.Since(start), "err", err != nil)
	}
	if err != nil {
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return protocol.ResponseHeader{}, gameerr.Network(fmt.Errorf("http 状态码 %d", resp.StatusCode))
	}
	// gzip 由 http.Transport 透明处理（未手动设置 Accept-Encoding）。
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}
	c.localTime = time.Now()

	var header protocol.ResponseHeader
	if err := decodeEnvelope(raw, crypted, &header, out); err != nil {
		// 与原项目一致：解码失败视为网络异常。
		return protocol.ResponseHeader{}, gameerr.Network(err)
	}

	// 同步服务器时间
	if header.ServerTime != 0 {
		c.serverTime = header.ServerTime
	} else {
		c.serverTime = c.localTime.Unix()
	}
	// 更新会话头 / viewer_id
	if header.SID != "" {
		c.headers["SID"] = sidHash(header.SID)
	}
	if header.RequestID != "" {
		c.headers["REQUEST-ID"] = header.RequestID
	}
	if header.ViewerID != "" {
		if v, perr := strconv.ParseInt(header.ViewerID, 10, 64); perr == nil {
			c.viewerID = v
		}
	}

	// TODO(后续): 维护状态响应 store_url 的版本自动升级；此处仅捕获 header.StoreURL。

	// 业务错误
	if ec, ok := out.(protocol.ErrorCarrier); ok {
		if se := ec.GetServerError(); se != nil {
			c.logger.Error("game api error", "url", req.URL(), "result_code", header.ResultCode, "message", se.Message)
			if header.ResultCode == 203 {
				return header, gameerr.Panic("%s", se.Message)
			}
			if len(c.servers) > 0 {
				c.active = (c.active + 1) % len(c.servers)
			}
			return header, &gameerr.APIError{Message: se.Message, Status: se.Status, ResultCode: header.ResultCode}
		}
	}

	return header, nil
}
