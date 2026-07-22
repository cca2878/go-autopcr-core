package asset

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/urlx"
)

// DefaultRes 是 B 服资源 CDN 根（对应 assetmgr.res）；下发为空时的兜底。
const DefaultRes = "https://l1-prod-patch-gzlj.bilibiligame.net/client_ob_771"

// masterdataURL 是 masterdata 资源包在清单中的逻辑 url。
const masterdataURL = "a/masterdata_master.unity3d"

// Source 从 CDN 解析清单并下载资源。
//
// res 是「目录形式」的 base URL（经 urlx.ParseBase 规整，末尾带 "/"），所有拼接统一
// 用 ResolveReference 相对解析（相对引用不带前导 "/"）。
type Source struct {
	res  *url.URL
	http *http.Client
}

// Option 定制 Source。
type Option func(*Source)

// WithHTTPClient 覆盖默认 http.Client（如与游戏 API 共享底层 Transport）。
func WithHTTPClient(h *http.Client) Option { return func(s *Source) { s.http = h } }

// WithRes 覆盖资源 CDN 根；res 应为经 urlx.ParseBase 规整的 base URL。
func WithRes(res *url.URL) Option { return func(s *Source) { s.res = res } }

// NewSource 构造一个资源源。
func NewSource(opts ...Option) *Source {
	s := &Source{res: urlx.MustParseBase(DefaultRes), http: &http.Client{Timeout: 60 * time.Second}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// manifestBase 返回某版本清单根（目录形式，末尾带 "/"，供相对解析子清单）。
func (s *Source) manifestBase(ver int) *url.URL {
	return s.res.ResolveReference(&url.URL{Path: "Manifest/AssetBundles/Android/" + strconv.Itoa(ver) + "/"})
}

// Resolve 拉取并递归解析清单，返回 url→Content 注册表。
//
// 【必须走完整棵树】：同一个逻辑 url 会在多个子清单里重复出现，且以【最后一条】为准——
// 实测 a/masterdata_master.unity3d 先出现的那条给的是内容摘要、按它拼出的 pool 路径 404，
// 最后一条给的才是 pool 键。任何「命中即停」的优化都会取错条目。
func (s *Source) Resolve(ctx context.Context, ver int) (map[string]*Content, error) {
	registry := make(map[string]*Content)
	if err := s.resolveManifest(ctx, s.manifestBase(ver), "manifest/manifest_assetmanifest", "AssetBundles/Android", registry, nil); err != nil {
		return nil, err
	}
	return registry, nil
}

// resolveManifest 递归展开一张清单。ancestors 是当前递归路径上的清单集合，仅用于防环：
// 清单内容由服务端下发，自引用/互引用会让这里无限递归下去（每层还附带一次 HTTP 拉取）。
// 注意只能按【路径】去重而非全局去重——同一张子清单在树中被引用多次是合法的，跳过重复展开
// 会改变「后者覆盖前者」的最终取值。
func (s *Source) resolveManifest(ctx context.Context, base *url.URL, ref, category string, registry map[string]*Content, ancestors map[string]bool) error {
	text, err := s.getText(ctx, base.ResolveReference(&url.URL{Path: ref}))
	if err != nil {
		return fmt.Errorf("拉取清单 %s: %w", ref, err)
	}
	if ancestors == nil {
		ancestors = map[string]bool{}
	}
	ancestors[ref] = true
	defer delete(ancestors, ref)

	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		c, ok := parseLine(line, category)
		if !ok {
			continue
		}
		registry[c.URL] = c
		if c.isManifest() && !ancestors[c.URL] {
			if err := s.resolveManifest(ctx, base, c.URL, category, registry, ancestors); err != nil {
				return err
			}
		}
	}
	return nil
}

// Download 下载一个资源条目的原始字节（pool/{category}/{md5[:2]}/{md5}）。
func (s *Source) Download(ctx context.Context, c *Content) ([]byte, error) {
	if len(c.MD5) < 2 {
		return nil, fmt.Errorf("无效 md5: %q", c.MD5)
	}
	// 注意 c.MD5 是 pool 的【寻址键】，不保证等于内容摘要：实测 masterdata 条目的该字段为
	// 16 位十六进制，而下下来的内容 md5 是另一个 32 位值。故此处不能拿它当校验和。
	ref := &url.URL{Path: "pool/" + c.Category + "/" + c.MD5[:2] + "/" + c.MD5}
	return s.getBytes(ctx, s.res.ResolveReference(ref))
}

// FetchMasterdata 解析清单、定位并下载 masterdata_master.unity3d 的原始字节。
func (s *Source) FetchMasterdata(ctx context.Context, ver int) ([]byte, error) {
	registry, err := s.Resolve(ctx, ver)
	if err != nil {
		return nil, err
	}
	c, ok := registry[masterdataURL]
	if !ok {
		return nil, fmt.Errorf("清单中未找到 %s", masterdataURL)
	}
	return s.Download(ctx, c)
}

func (s *Source) getText(ctx context.Context, u *url.URL) (string, error) {
	b, err := s.getBytes(ctx, u)
	return string(b), err
}

func (s *Source) getBytes(ctx context.Context, u *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: 状态 %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
