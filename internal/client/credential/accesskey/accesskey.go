// Package accesskey 实现"AccessKey 四要素直传"的凭据，是 credential 端口的核心内实现。
//
// 它不做任何账密登录：Login 直接返回构造时传入的 uid/access_key；渠道（bsdk/qsdk）
// 仅决定 apiRoot / resKey / platformID / channelID 等静态配置与请求头。
package accesskey

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"maps"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 渠道标识。
const (
	ChannelBSDK = "bsdk" // 官服（免登录，直传 AccessKey）
	ChannelQSDK = "qsdk" // 渠道服
)

// 凭据构造与使用的三种失败。都是'调用方传错了东西'而非运行期意外，故同归 errs.KindMisuse：
// 外壳判定后直接把用户引导到对应的输入项即可，不必去猜错误文本。
var (
	// ErrUnknownChannel 表示渠道标识不是 ChannelBSDK / ChannelQSDK 之一。
	ErrUnknownChannel = errs.DomainCredential.New(errs.KindMisuse, "未知渠道")

	// ErrEmptyCredential 表示 uid 或 access_key 为空。
	ErrEmptyCredential = errs.DomainCredential.New(errs.KindMisuse, "uid 与 access_key 均不能为空")

	// ErrAnonymousLogin 表示拿只供免凭证握手用的匿名凭据去走登录序列。这是装配错误：
	// 匿名凭据没有 uid，本就不可能登录成功。
	ErrAnonymousLogin = errs.DomainCredential.New(errs.KindMisuse, "匿名凭据不可用于登录")
)

// Android 平台标识（PLATFORM 头 / DEVICE 头）。
const platformAndroid = "2"

// androidHeaders 复刻原 constants.py 的 DEFAULT_HEADERS（Android）。
//
// APP-VER 只是'出厂默认值'、会过期：游戏更新后服务端拒绝旧版本号，transport.Client 会从
// 拒绝响应的 store_url 自动读出真实版本号纠正，故这里的值不必跟着每次更新维护。
//
// User-Agent / X-Unity-Version 没有这套自愈（服务端目前未观察到校验它们），只能手动跟版本更新
// 时取证。取证方法：真机 APK 的 libunity.so 里能搜到形如 "{短版本}/respin/{完整版本}-{commit}"
// 的构建标签字符串（Unity 自身写入的构建标识），以及 "libcurl/{版本}"；User-Agent 遵循
// libunity.so 里的模板 "UnityPlayer/%s (UnityWebRequest/1.0, %s)" 拼出。当前值对应 Unity 6
// （6000.0.58f2）与 libcurl 8.10.1；UnityWebRequest 版本号固定为 1.0。
//
// 注：不含 Accept-Encoding —— Go 的 http.Transport 未手动指定时会自动添加
// "Accept-Encoding: gzip"（规范大小写，与官方客户端一致）并透明解压响应。
var androidHeaders = map[string]string{
	"User-Agent":           "UnityPlayer/6000.0.58f2 (UnityWebRequest/1.0, libcurl/8.10.1)",
	"X-Unity-Version":      "6000.0.58f2",
	"APP-VER":              "11.7.2",
	"BATTLE-LOGIC-VERSION": "4",
	"BUNDLE-VER":           "",
	"DEVICE":               "2",
	"DEVICE-NAME":          "OPPO PCRT00",
	"EXCEL-VER":            "1.0.0",
	"GRAPHICS-DEVICE-NAME": "Adreno (TM) 640",
	"IP-ADDRESS":           "10.0.2.15",
	"KEYCHAIN":             "",
	"LOCALE":               "CN",
	"PLATFORM-OS-VERSION":  "Android OS 5.1.1 / API-22 (LMY48Z/rel.se.infra.20200612.100533)",
	"REGION-CODE":          "",
	"RES-VER":              "10002200",
	"SHORT-UDID":           "0",
}

// channelConfig 是各渠道的静态配置。
type channelConfig struct {
	apiRoot    string
	resKey     string
	platformID string // ToolSdkLogin 的 platform，也用于 PLATFORM-ID 头
	channelID  string // ToolSdkLogin 的 channel_id，也用于 CHANNEL-ID 头
}

var channels = map[string]channelConfig{
	ChannelBSDK: {
		apiRoot:    "https://l3-prod-all-gs-gzlj.bilibiligame.net/",
		resKey:     "ab00a0a6dd915a052a2ef7fd649083e5",
		platformID: "2",
		channelID:  "1",
	},
	ChannelQSDK: {
		apiRoot:    "https://l1-prod-uo-gs-gzlj.bilibiligame.net/",
		resKey:     "d145b29050641dac2f8b19df0afe0e59",
		platformID: "4",
		channelID:  "1",
	},
}

// Credential 是 AccessKey 直传凭据。
//
// solver 可选：核心不携带任何验证码求解器实现，默认为 nil。未注入求解器时，仅在真正触发
// 风控(is_risk)才会以 captcha.ErrNoSolver 硬失败——绝大多数正常登录不需要求解器。求解能力
// 由外壳经 WithCaptchaSolver 注入。
type Credential struct {
	uid       string
	accessKey string
	cfg       channelConfig
	solver    captcha.Solver // 可为 nil：未注入求解器
}

// Option 用于定制 Credential。
type Option func(*Credential)

// WithCaptchaSolver 注入验证码求解器（外壳侧构造，如 gtrv 远程或本地 wasm）。
// 不注入则触发风控时以 captcha.ErrNoSolver 硬失败。
func WithCaptchaSolver(s captcha.Solver) Option {
	return func(c *Credential) { c.solver = s }
}

// New 构造一个直传凭据。channel 取 ChannelBSDK / ChannelQSDK。
func New(channel, uid, accessKey string, opts ...Option) (*Credential, error) {
	cfg, ok := channels[channel]
	if !ok {
		return nil, fmt.Errorf("%w %q（支持 %q / %q）", ErrUnknownChannel, channel, ChannelBSDK, ChannelQSDK)
	}
	if uid == "" || accessKey == "" {
		return nil, ErrEmptyCredential
	}
	c := &Credential{
		uid:       uid,
		accessKey: accessKey,
		cfg:       cfg,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Anonymous 构造仅用于'免凭证握手'(source_ini/index + get_maintenance_status，均非加密)
// 的凭据：只提供渠道静态配置(APIRoot/Header/platform/channel)，不含账号 uid/access_key。
//
// 供 masterdata 无凭证刷新用——不登录也能拉服务器列表与维护状态、进而取最新母数据。
// Login 会返回错误：它绝不应被用于真正的登录序列。
func Anonymous(channel string) (*Credential, error) {
	cfg, ok := channels[channel]
	if !ok {
		return nil, fmt.Errorf("%w %q（支持 %q / %q）", ErrUnknownChannel, channel, ChannelBSDK, ChannelQSDK)
	}
	return &Credential{cfg: cfg}, nil
}

// Login 直接返回构造时传入的 uid / access_key；匿名凭据（无 uid）不可登录。
func (c *Credential) Login(ctx context.Context) (string, string, error) {
	if c.uid == "" {
		return "", "", ErrAnonymousLogin
	}
	return c.uid, c.accessKey, nil
}

// Header 返回本渠道的基础请求头。
func (c *Credential) Header() map[string]string {
	h := maps.Clone(androidHeaders)
	h["DEVICE-ID"] = md5Hex(c.uid)
	h["RES-KEY"] = c.cfg.resKey
	h["PLATFORM"] = platformAndroid
	h["PLATFORM-ID"] = c.cfg.platformID
	h["CHANNEL-ID"] = c.cfg.channelID
	return h
}

// APIRoot 返回本渠道 API 根地址。
func (c *Credential) APIRoot() string { return c.cfg.apiRoot }

// PlatformID 返回 ToolSdkLogin 的 platform 值。
func (c *Credential) PlatformID() string { return c.cfg.platformID }

// ChannelID 返回 ToolSdkLogin 的 channel_id 值。
func (c *Credential) ChannelID() string { return c.cfg.channelID }

// DoCaptcha 委托给注入的验证码求解器；未注入时返回 captcha.ErrNoSolver（硬失败）。
func (c *Credential) DoCaptcha(ctx context.Context) (*captcha.Result, error) {
	if c.solver == nil {
		return nil, captcha.ErrNoSolver
	}
	return c.solver.Solve(ctx)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
