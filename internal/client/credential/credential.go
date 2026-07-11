// Package credential 定义无头客户端向下依赖的凭据端口（SC 缝）。
//
// 游戏服务器鉴权只需四要素 uid/access_key/platform/channel_id；bsdk/qsdk 等
// 「渠道」只是获取 access_key 的手段与静态配置来源。核心唯一实现为「AccessKey 四要素
// 直传」（见子包 accesskey）——核心只吃 (channel, uid, access_key)。
//
// 账密→access_key 属【冷启动】、是外壳(imperative shell)职责：核心不携带登录 SDK、
// 也不依赖 bsdkv3-go（见架构决策：SDK 移出核心）。外壳完成账密登录后，仍以
// accesskey.Credential 喂本端口，故对 transport/session 完全透明。
package credential

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
)

// Credential 是凭据端口：向传输/会话层提供鉴权信息、请求头与渠道配置。
type Credential interface {
	// Login 返回鉴权用的 uid 与 access_key。
	//
	// 直传实现直接返回构造时传入的值；未来账密实现会在此发起真实登录。
	Login(ctx context.Context) (uid, accessKey string, err error)

	// Header 返回本渠道的基础请求头（含 DEVICE-ID/RES-KEY/PLATFORM/PLATFORM-ID/CHANNEL-ID）。
	Header() map[string]string

	// APIRoot 返回本渠道的 API 根地址（登录序列起点）。
	APIRoot() string

	// PlatformID 返回 ToolSdkLogin 请求的 platform 值。
	PlatformID() string

	// ChannelID 返回 ToolSdkLogin 请求的 channel_id 值。
	ChannelID() string

	// DoCaptcha 求解风控验证码：委托给注入的求解器；未注入时返回 captcha.ErrNoSolver（硬失败）。
	DoCaptcha(ctx context.Context) (*captcha.Result, error)
}
