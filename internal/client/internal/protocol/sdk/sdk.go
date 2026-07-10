// Package sdk 是「登录握手/SDK」域的 DTO（source_ini、tool/sdk_login、check/game_start；
// 对应 ref model/sdkrequests.py）。供 session 登录序列使用。
package sdk

import (
	"net/url"

	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
)

// 端点相对引用（解析一次，复用；ResolveReference 只读引用、不修改，故共享安全）。
var (
	urlSourceIniIndex       = protocol.MustRelURL("source_ini/index?format=json")
	urlSourceIniMaintenance = protocol.MustRelURL("source_ini/get_maintenance_status?format=json")
	urlToolSdkLogin         = protocol.MustRelURL("tool/sdk_login")
	urlCheckGameStart       = protocol.MustRelURL("check/game_start")
)

// SourceIniIndexRequest 获取真实服务器列表（明文 JSON）。
type SourceIniIndexRequest struct {
	protocol.RequestBase
}

func (*SourceIniIndexRequest) URL() *url.URL { return urlSourceIniIndex }
func (*SourceIniIndexRequest) Crypted() bool { return false }

// SourceIniIndexResponse 携带服务器地址列表。
type SourceIniIndexResponse struct {
	protocol.ResponseBase
	Server []string `msgpack:"server" json:"server"`
}

// SourceIniGetMaintenanceStatusRequest 获取维护/版本状态（明文 JSON）。
type SourceIniGetMaintenanceStatusRequest struct {
	protocol.RequestBase
}

func (*SourceIniGetMaintenanceStatusRequest) URL() *url.URL { return urlSourceIniMaintenance }
func (*SourceIniGetMaintenanceStatusRequest) Crypted() bool { return false }

// SourceIniGetMaintenanceStatusResponse 携带资源/清单版本等信息。
type SourceIniGetMaintenanceStatusResponse struct {
	protocol.ResponseBase
	ResVer              string   `msgpack:"res_ver" json:"res_ver"`
	ManifestVer         string   `msgpack:"manifest_ver" json:"manifest_ver"`
	RequiredManifestVer string   `msgpack:"required_manifest_ver" json:"required_manifest_ver"`
	MaintenanceMessage  string   `msgpack:"maintenance_message" json:"maintenance_message"`
	ResHTTPType         int      `msgpack:"res_http_type" json:"res_http_type"`
	Resource            []string `msgpack:"resource" json:"resource"`
}

// ToolSdkLoginRequest 用鉴权四要素登录游戏服。
//
// 验证码相关字段仅在触发风控(is_risk)时才填充（由 session.passRisk 带票据重提交，求解器由外壳
// 注入）；未触发时以指针 nil 编码为 msgpack nil，与原项目 use_bin_type=False 的 None 一致。
type ToolSdkLoginRequest struct {
	protocol.RequestBase
	UID         string  `msgpack:"uid" json:"uid"`
	AccessKey   string  `msgpack:"access_key" json:"access_key"`
	Platform    string  `msgpack:"platform" json:"platform"`
	ChannelID   string  `msgpack:"channel_id" json:"channel_id"`
	Challenge   *string `msgpack:"challenge" json:"challenge"`
	Validate    *string `msgpack:"validate" json:"validate"`
	Seccode     *string `msgpack:"seccode" json:"seccode"`
	CaptchaType *string `msgpack:"captcha_type" json:"captcha_type"`
	ImageToken  *string `msgpack:"image_token" json:"image_token"`
	CaptchaCode *string `msgpack:"captcha_code" json:"captcha_code"`
}

func (*ToolSdkLoginRequest) URL() *url.URL { return urlToolSdkLogin }

// ToolSdkLoginResponse 表示登录结果；is_risk 为真时需验证码。
type ToolSdkLoginResponse struct {
	protocol.ResponseBase
	IsRisk bool `msgpack:"is_risk" json:"is_risk"`
}

// CheckGameStartRequest 校验游戏启动状态。
type CheckGameStartRequest struct {
	protocol.RequestBase
	AppType      int    `msgpack:"apptype" json:"apptype"`
	CampaignData string `msgpack:"campaign_data" json:"campaign_data"`
	CampaignUser int    `msgpack:"campaign_user" json:"campaign_user"`
}

func (*CheckGameStartRequest) URL() *url.URL { return urlCheckGameStart }

// CheckGameStartResponse 中 now_tutorial 为假表示账号未过完教程。
type CheckGameStartResponse struct {
	protocol.ResponseBase
	NowTutorial bool `msgpack:"now_tutorial" json:"now_tutorial"`
}
