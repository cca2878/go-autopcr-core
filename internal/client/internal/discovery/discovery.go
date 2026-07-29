// Package discovery 封装【免凭证服务端发现握手】：source_ini/index + get_maintenance_status。
//
// 这两步均为非加密、免凭证请求（Crypted()==false），故可用匿名凭据构造的 transport 执行。
// 登录序列（session）与母数据无凭证刷新（masterdata）共用本原语，避免重复实现握手与 res 解析。
package discovery

import (
	"context"
	"net/url"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/urlx"
)

// Result 是握手拿到的服务端发现信息。
type Result struct {
	Servers             []*url.URL // 真实游戏服务器 base URL 列表
	ResVer              string     // 资源版本
	ManifestVer         string     // 清单版本（母数据 ensure 用；对应 manifest_ver）
	RequiredManifestVer string     // 需要的清单版本（会话 MANIFEST-VER 头用）
	// ResURLs 是下发的【全部】资源 CDN 根（按 res_http_type 定 scheme），顺序即下发顺序。
	// 实测下发三台（l1/l3/l4），互为备份：首台故障时可换下一台。下发空/全部非法时为空切片。
	ResURLs []*url.URL
}

// Discover 执行 source_ini/index → get_maintenance_status，返回发现信息。
//
// 会在拿到服务器列表后对 c 调用 SetServers（维护请求须打到真实服务器）；除此之外不改动
// 会话其它状态（MANIFEST-VER 头等由调用方按需设置）。传入的 transport.Client 可用匿名凭据。
func Discover(ctx context.Context, c *transport.Client) (*Result, error) {
	idx, err := transport.Call[sdk.SourceIniIndexResponse](ctx, c, &sdk.SourceIniIndexRequest{})
	if err != nil {
		return nil, err
	}
	servers := normalizeServers(idx.Server)
	if len(servers) == 0 {
		return nil, gameerr.Panic("服务器列表为空")
	}
	c.SetServers(servers)

	mnt, err := transport.Call[sdk.SourceIniGetMaintenanceStatusResponse](ctx, c, &sdk.SourceIniGetMaintenanceStatusRequest{})
	if err != nil {
		return nil, err
	}
	return &Result{
		Servers:             servers,
		ResVer:              mnt.ResVer,
		ManifestVer:         mnt.ManifestVer,
		RequiredManifestVer: mnt.RequiredManifestVer,
		ResURLs:             ResolveResURLs(mnt.ResHTTPType, mnt.Resource),
	}, nil
}

// normalizeServers 复刻原项目 f'https://{server}'.replace('\t',”)，并解析为 base URL。
// 无法解析的条目跳过（与 empty 条目相同的容错）。
func normalizeServers(servers []string) []*url.URL {
	out := make([]*url.URL, 0, len(servers))
	for _, s := range servers {
		s = strings.ReplaceAll(s, "\t", "")
		if s == "" {
			continue
		}
		u, err := urlx.ParseBase("https://" + s)
		if err != nil {
			continue
		}
		out = append(out, u)
	}
	return out
}

// ResolveResURLs 把维护响应下发的 resource 列表解析为资源 CDN 根 URL 列表，【保留全部主机】。
//
// 下发项形如 "l1-xxx-gzlj.bilibiligame.net/client_ob_771/"（无 scheme、带尾斜杠），
// scheme 由 res_http_type 决定（实测 0=https）。无法解析的条目跳过（与 normalizeServers
// 同样的容错）；全部为空/非法时返回空切片，由上层回退到内置默认 CDN。
//
// 保留全部而非只取首个：服务端下发多台正是为了互为备份（实测 l1/l3/l4 三台），只留一台
// 等于把它们的冗余丢掉——首台一挂就只能回退到内置默认 CDN，而那个地址是写死的、更旧。
func ResolveResURLs(httpType int, resource []string) []*url.URL {
	scheme := "https"
	if httpType != 0 {
		scheme = "http"
	}
	out := make([]*url.URL, 0, len(resource))
	for _, host := range resource {
		host = strings.TrimRight(strings.ReplaceAll(host, "\t", ""), "/")
		if host == "" {
			continue
		}
		u, err := urlx.ParseBase(scheme + "://" + host)
		if err != nil {
			continue
		}
		out = append(out, u)
	}
	return out
}
