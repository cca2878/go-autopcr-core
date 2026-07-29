package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
// res 是「目录形式」的 base URL 列表（各项经 urlx.ParseBase 规整，末尾带 "/"），所有拼接
// 统一用 ResolveReference 相对解析（相对引用不带前导 "/"）。
//
// 多个 res 互为备份：服务端下发多台正是这个用途（实测 l1/l3/l4）。一台失败时按序换下一台，
// 但只在「换一台可能有救」时才换——见 worthAnotherHost。
type Source struct {
	res    []*url.URL
	http   *http.Client
	logger *slog.Logger
}

// Option 定制 Source。
type Option func(*Source)

// WithHTTPClient 覆盖默认 http.Client（如与游戏 API 共享底层 Transport）。
func WithHTTPClient(h *http.Client) Option { return func(s *Source) { s.http = h } }

// WithRes 覆盖资源 CDN 根（单台）；res 应为经 urlx.ParseBase 规整的 base URL。
func WithRes(res *url.URL) Option {
	return func(s *Source) {
		if res != nil {
			s.res = []*url.URL{res}
		}
	}
}

// WithResList 覆盖资源 CDN 根列表（多台互为备份，按序尝试）。nil 项与空列表被忽略——
// 保留原有取值，以免下发缺失时把唯一可用的兜底也清掉。
func WithResList(list []*url.URL) Option {
	return func(s *Source) {
		out := make([]*url.URL, 0, len(list))
		for _, u := range list {
			if u != nil {
				out = append(out, u)
			}
		}
		if len(out) > 0 {
			s.res = out
		}
	}
}

// WithLogger 设置日志器（默认 slog.Default()）：故障转移换主机时会记一条 warn，
// 否则这类自愈完全隐形、事后无从排查。
func WithLogger(l *slog.Logger) Option {
	return func(s *Source) {
		if l != nil {
			s.logger = l
		}
	}
}

// NewSource 构造一个资源源。
func NewSource(opts ...Option) *Source {
	s := &Source{
		res:    []*url.URL{urlx.MustParseBase(DefaultRes)},
		http:   &http.Client{Timeout: 60 * time.Second},
		logger: slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// manifestBase 返回某版本在 root 上的清单根（目录形式，末尾带 "/"，供相对解析子清单）。
func (s *Source) manifestBase(root *url.URL, ver int) *url.URL {
	return root.ResolveReference(&url.URL{Path: "Manifest/AssetBundles/Android/" + strconv.Itoa(ver) + "/"})
}

// worthAnotherHost 报告这次失败是否值得换一台主机再试。
//
// 判据就是 HTTPError.Retryable：5xx/429 是这台的事，404 换到哪台都一样。非 HTTP 类的失败
// （连不上、超时、读到一半断了）同样值得换台试——除非调用方已经取消，那时换谁都是空转。
func worthAnotherHost(err error) bool {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Retryable()
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// Resolve 拉取并递归解析清单，返回 url→Content 注册表。
//
// 【必须走完整棵树】：同一个逻辑 url 会在多个子清单里重复出现，且以【最后一条】为准——
// 实测 a/masterdata_master.unity3d 先出现的那条给的是内容摘要、按它拼出的 pool 路径 404，
// 最后一条给的才是 pool 键。任何「命中即停」的优化都会取错条目。
// 故障转移以【整棵树】为单位：一台不成就换下一台从头解析，而不是中途接着往下走。因为同一
// 个逻辑 url 会在多个子清单里重复出现、以最后一条为准（见上），半棵树来自 A、半棵来自 B 时
// 这个「谁最后」就跨了两台主机的内容，取出来的可能是任何一条。整棵重来只多花一次清单拉取。
func (s *Source) Resolve(ctx context.Context, ver int) (map[string]*Content, error) {
	var lastErr error
	for i, root := range s.res {
		registry := make(map[string]*Content)
		err := s.resolveManifest(ctx, s.manifestBase(root, ver), "manifest/manifest_assetmanifest", "AssetBundles/Android", registry, nil)
		if err == nil {
			return registry, nil
		}
		lastErr = err
		if !worthAnotherHost(err) || i == len(s.res)-1 {
			break
		}
		s.logger.Warn("清单解析失败，改用下一台资源 CDN",
			"host", root.Host, "next", s.res[i+1].Host, "err", err)
	}
	return nil, lastErr
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
		return nil, fmt.Errorf("%w：条目 %s 的 md5 键为 %q", ErrBadManifest, c.URL, c.MD5)
	}
	// 注意 c.MD5 是 pool 的【寻址键】，不保证等于内容摘要：实测 masterdata 条目的该字段为
	// 16 位十六进制，而下下来的内容 md5 是另一个 32 位值。故此处不能拿它当校验和。
	ref := &url.URL{Path: "pool/" + c.Category + "/" + c.MD5[:2] + "/" + c.MD5}
	return s.fetchAcrossHosts(ctx, ref)
}

// fetchAcrossHosts 在各候选主机上依次尝试取同一个相对引用。
//
// 与 Resolve 的整棵树重来不同，单个资源没有跨主机的顺序语义，逐台试即可。
func (s *Source) fetchAcrossHosts(ctx context.Context, ref *url.URL) ([]byte, error) {
	var lastErr error
	for i, root := range s.res {
		b, err := s.getBytes(ctx, root.ResolveReference(ref))
		if err == nil {
			return b, nil
		}
		lastErr = err
		if !worthAnotherHost(err) || i == len(s.res)-1 {
			break
		}
		s.logger.Warn("资源下载失败，改用下一台资源 CDN",
			"host", root.Host, "next", s.res[i+1].Host, "err", err)
	}
	return nil, lastErr
}

// FetchMasterdata 解析清单、定位并下载 masterdata_master.unity3d 的原始字节。
func (s *Source) FetchMasterdata(ctx context.Context, ver int) ([]byte, error) {
	registry, err := s.Resolve(ctx, ver)
	if err != nil {
		return nil, err
	}
	c, ok := registry[masterdataURL]
	if !ok {
		return nil, fmt.Errorf("%w：%s", ErrNotInManifest, masterdataURL)
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
		return nil, &HTTPError{URL: u.String(), Status: resp.StatusCode}
	}
	return io.ReadAll(resp.Body)
}
