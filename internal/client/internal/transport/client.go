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
	"regexp"
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

	handler         Handler
	onVersionUpdate func(newAppVer string)
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

// WithOnVersionUpdate 注册 APP-VER 自愈成功时的回调（见 transport 方法）。newAppVer 是纠正后的
// 版本号。本包不做任何持久化——是否落盘、落到哪里由调用方决定（见 appversion 包），这里只负责
// 在【状态确实变化的那一刻】通知它，不多不少。未注册则自愈仍然发生，只是没有旁路通知。
func WithOnVersionUpdate(fn func(newAppVer string)) Option {
	return func(c *Client) { c.onVersionUpdate = fn }
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

// storeURLVersionPattern 从维护状态响应下发的 store_url（应用商店安装包链接）中提取真实版本号。
// 复刻原项目 apiclient.py 的同名正则；Go 的 regexp（RE2）不支持环视，故用捕获组取代 lookbehind。
// 形如 https://pkg.biligame.com/games/gzlj_11.7.2_20260715_154600_b8233_896629.apk。
var storeURLVersionPattern = regexp.MustCompile(`gzlj_(\d+\.\d+\.\d+)`)

// parseStoreURLVersion 从 store_url 中解析出真实版本号；解不出（字段为空或形状不符）返回 false。
func parseStoreURLVersion(storeURL string) (string, bool) {
	m := storeURLVersionPattern.FindStringSubmatch(storeURL)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// transport 是最内层处理器：单次尝试之外附带 APP-VER 过期自愈。
//
// 客户端版本号（APP-VER 头）落后于服务端认可的版本时，服务端会拒绝请求并在 store_url 里下发
// 当前安装包链接，从中能读出真实版本号（对应原项目 apiclient.py 的 store_url 探测）。本包自己
// 不做持久化（是否落盘、落到哪里是调用方的事，见 WithOnVersionUpdate），但会在纠正发生的那一刻
// 通知已注册的回调，使调用方能把它记下来，避免下一个新进程还要再吃一次这次的多余往返。
//
// 必须在这里而非外层中间件处理：版本不符时 result_code=204、status=3，与「会话失效」共用同一个
// status（见 relogin.go 的 statusSessionInvalid），若放任它冒泡到 session guard，会被误判成需要
// 重登——而重登发出的请求带着同样过期的头，会在这里再栽一次跟头。只重试一次：纠正后仍失败，
// 就不是版本的事，照常按原样上抛给外层处理。
func (c *Client) transport(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
	header, err := c.doTransport(ctx, req, out)

	newVer, ok := parseStoreURLVersion(header.StoreURL)
	if !ok {
		return header, err
	}
	c.mu.Lock()
	cur := c.headers["APP-VER"]
	c.mu.Unlock()
	if newVer == cur {
		return header, err
	}
	c.logger.Info("APP-VER 已过期，自动升级并重试请求", "url", req.URL(), "old", cur, "new", newVer)
	c.SetHeader("APP-VER", newVer)
	if c.onVersionUpdate != nil {
		c.onVersionUpdate(newVer)
	}
	ZeroResponse(out)
	return c.doTransport(ctx, req, out)
}

// doTransport 是单次尝试：加密/编码/HTTP/解码/维护会话头。
func (c *Client) doTransport(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
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
		// 与原项目一致：解码失败视为网络异常，从而落入 ErrorHandler 的重试。包在里面的是
		// gameerr.ProtocolError，要分辨「链路不通」还是「响应形状对不上」再 As 一次即可。
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

	// 业务错误
	if ec, ok := out.(protocol.ErrorCarrier); ok {
		if se := ec.GetServerError(); se != nil {
			// 只按响应码短路；维护消息照常上抛，由 errorhandler 中间件升级
			// （分层理由见 gameerr.IsFatalBusiness）。
			if gameerr.IsFatalResultCode(header.ResultCode) {
				// 这一档【确定不可恢复】，本层就是终点，记 Error 名副其实。
				c.logger.Error("游戏服返回不可恢复的业务错误",
					"url", req.URL(), "result_code", header.ResultCode, "message", se.Message)
				return header, gameerr.Panic("%s", se.Message)
			}
			// 其余一律 Warn：本层【不知道后果】。会话失效那一类紧接着就被 sessionGuard 自愈了，
			// 记成 Error 会让一次成功的自愈在日志里留下一条吓人的错误；模块的业务错误则会变成
			// 该任务的 Result.Err，由调用方决定它算不算失败。谁知道后果，谁记 Error。
			c.logger.Warn("游戏服返回业务错误",
				"url", req.URL(), "result_code", header.ResultCode, "message", se.Message)
			if len(c.servers) > 0 {
				c.active = (c.active + 1) % len(c.servers)
			}
			return header, &gameerr.APIError{Message: se.Message, Status: se.Status, ResultCode: header.ResultCode}
		}
	}

	return header, nil
}
