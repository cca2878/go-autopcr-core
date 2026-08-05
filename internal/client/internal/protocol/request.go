// Package protocol 定义游戏 API 的请求/响应'共享基座'：Request 契约、RequestBase/
// ResponseBase、ResponseHeader、错误类型、URL 助手，以及跨域共用的响应模型。
//
// 具体 DTO 按游戏功能域拆到子包（protocol/sdk、protocol/account、protocol/daily…），各子包
// 内嵌本包的 RequestBase/ResponseBase 并用 MustRelURL 声明端点。DTO 数量最多，按域分包便于
// 组织与增量扩充。
//
// # 共用模型的落位规则
//
// 一个响应模型只要'跨 2 个及以上域包'出现，就定义在本包，不在域包各写一份。判据不是"现在
// 有几处在用"而是"协议上它是否跨域"——服务端把同一个形状挂在各域响应下，域包各写一份的
// 结果就是同一个东西有 N 个名字 N 份定义（本库曾有 7 份 InventoryInfo，字段还各不相同）。
//
// 按域拆分的文件：currency.go（金币/钻石）、inventory.go（库存条目）、player.go（体力/等级）、
// unit.go（角色）、quest.go（战斗结算）。新增共用模型时归入对应文件，都不沾边再开新文件。
//
// 请求体的 msgpack 编码由 transport 包的兼容编码器完成（复刻参考项目 use_bin_type=False 的语
// 义）；响应用标准 msgpack/JSON 解码。
package protocol

import "net/url"

// Request 是所有游戏 API 请求的契约。
type Request interface {
	// URL 返回相对于服务器根的端点引用（相对 URL）。transport 用服务器 base URL
	// 的 ResolveReference 拼成绝对地址，故此处应返回不含 scheme/host 的相对引用。
	URL() *url.URL
	// Crypted 表示是否走加密通道（msgpack + AES）；否则走明文 JSON。
	Crypted() bool
	// SetViewerID 在发送前由 transport 注入 viewer_id 字段。
	SetViewerID(v string)
}

// MustRelURL 解析一个相对端点引用；供各 DTO 子包声明端点常量，解析失败即 panic（编程错误）。
func MustRelURL(ref string) *url.URL {
	u, err := url.Parse(ref)
	if err != nil {
		panic("protocol: 非法端点引用 " + ref + ": " + err.Error())
	}
	return u
}

// RequestBase 提供 viewer_id 字段与默认加密标志，供具体请求内嵌。
//
// 具体请求需内嵌 RequestBase 并自行实现 URL()；如需明文通道则覆盖 Crypted()。
type RequestBase struct {
	ViewerID string `msgpack:"viewer_id" json:"viewer_id"`
}

// SetViewerID 实现 Request 接口。
func (r *RequestBase) SetViewerID(v string) { r.ViewerID = v }

// Crypted 默认走加密通道；明文请求（如 source_ini）覆盖它返回 false。
func (r *RequestBase) Crypted() bool { return true }
