package asset

import (
	"context"
	"crypto/md5"
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
func (s *Source) Resolve(ctx context.Context, ver int) (map[string]*Content, error) {
	return s.resolve(ctx, ver, "")
}

// resolve 解析清单树。want 非空时一旦收录该 url 即停止展开余下子清单——清单树有几十个子清单、
// 逐个拉取是串行 HTTP，而我们通常只为了取其中一个条目。
func (s *Source) resolve(ctx context.Context, ver int, want string) (map[string]*Content, error) {
	registry := make(map[string]*Content)
	if err := s.resolveManifest(ctx, s.manifestBase(ver), "manifest/manifest_assetmanifest", "AssetBundles/Android", registry, want); err != nil {
		return nil, err
	}
	return registry, nil
}

func (s *Source) resolveManifest(ctx context.Context, base *url.URL, ref, category string, registry map[string]*Content, want string) error {
	text, err := s.getText(ctx, base.ResolveReference(&url.URL{Path: ref}))
	if err != nil {
		return fmt.Errorf("拉取清单 %s: %w", ref, err)
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		c, ok := parseLine(line, category)
		if !ok {
			continue
		}
		// 已见过的条目不再展开：清单内容由服务端下发，自引用/互引用会让这里无限递归下去
		// （每层还附带一次 HTTP 拉取），最终撑爆调用栈。registry 天然就是「已访问」集合。
		_, seen := registry[c.URL]
		registry[c.URL] = c
		if want != "" && c.URL == want {
			return nil
		}
		if c.isManifest() && !seen {
			if err := s.resolveManifest(ctx, base, c.URL, category, registry, want); err != nil {
				return err
			}
			if _, done := registry[want]; want != "" && done {
				return nil
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
	ref := &url.URL{Path: "pool/" + c.Category + "/" + c.MD5[:2] + "/" + c.MD5}
	b, err := s.getBytes(ctx, s.res.ResolveReference(ref))
	if err != nil {
		return nil, err
	}
	// 校验清单给出的 md5：LZ4 block 格式自身不带校验和，坏字节能一路走到 SQLite 并被
	// EnsureDB 固化进版本缓存，此后每次启动都命中这份坏库。这是唯一能拦住它的地方。
	if sum := fmt.Sprintf("%x", md5.Sum(b)); sum != c.MD5 {
		return nil, fmt.Errorf("资源 %s 校验失败：md5 %s != %s", c.URL, sum, c.MD5)
	}
	return b, nil
}

// FetchMasterdata 解析清单、定位并下载 masterdata_master.unity3d 的原始字节。
func (s *Source) FetchMasterdata(ctx context.Context, ver int) ([]byte, error) {
	registry, err := s.resolve(ctx, ver, masterdataURL)
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
