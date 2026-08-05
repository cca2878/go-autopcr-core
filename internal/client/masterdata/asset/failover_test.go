package asset

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/urlx"
)

// hostSpy 是一台假 CDN：按 status 应答，并记下被请求过几次。
type hostSpy struct {
	srv    *httptest.Server
	hits   int
	status int
	body   string
}

func newHostSpy(t *testing.T, status int, body string) *hostSpy {
	t.Helper()
	h := &hostSpy{status: status, body: body}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h.hits++
		w.WriteHeader(h.status)
		_, _ = w.Write([]byte(h.body))
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *hostSpy) base(t *testing.T) *url.URL {
	t.Helper()
	u, err := urlx.ParseBase(h.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func sourceOver(t *testing.T, hosts ...*hostSpy) *Source {
	t.Helper()
	list := make([]*url.URL, len(hosts))
	for i, h := range hosts {
		list[i] = h.base(t)
	}
	return NewSource(WithResList(list), WithHTTPClient(hosts[0].srv.Client()))
}

// 首台 5xx 时换下一台——这正是保留全部下发主机的意义：过去只取 resource[0]，它一挂就
// 只能回退到写死的内置 CDN。
func TestDownloadFailsOverOn5xx(t *testing.T) {
	down := newHostSpy(t, http.StatusServiceUnavailable, "")
	up := newHostSpy(t, http.StatusOK, "payload")

	got, err := sourceOver(t, down, up).Download(context.Background(),
		&Content{URL: "a/x.unity3d", Category: "AssetBundles/Android", MD5: "abcdef"})
	if err != nil {
		t.Fatalf("次选主机可用时不应失败：%v", err)
	}
	if string(got) != "payload" {
		t.Errorf("内容 = %q, want payload", got)
	}
	if down.hits != 1 || up.hits != 1 {
		t.Errorf("命中次数 down=%d up=%d，want 各 1 次", down.hits, up.hits)
	}
}

// 404 是"要的东西不在"：换台主机问还是不在，白跑一趟。这条把 HTTPError.Retryable 的
// 判据真正接进了故障转移——去掉它，下面的 second.hits 就会变成 1。
func TestDownloadDoesNotFailOverOn404(t *testing.T) {
	first := newHostSpy(t, http.StatusNotFound, "")
	second := newHostSpy(t, http.StatusOK, "payload")

	_, err := sourceOver(t, first, second).Download(context.Background(),
		&Content{URL: "a/x.unity3d", Category: "AssetBundles/Android", MD5: "abcdef"})
	if err == nil {
		t.Fatal("404 应直接失败")
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusNotFound {
		t.Fatalf("应为 404 的 HTTPError，得到 %v", err)
	}
	if second.hits != 0 {
		t.Errorf("404 不该触发换主机，次选被请求了 %d 次", second.hits)
	}
}

// 全部主机都不行时，返回最后一台的错误（而不是吞掉或返回 nil）。
func TestDownloadAllHostsDown(t *testing.T) {
	a := newHostSpy(t, http.StatusBadGateway, "")
	b := newHostSpy(t, http.StatusServiceUnavailable, "")

	_, err := sourceOver(t, a, b).Download(context.Background(),
		&Content{URL: "a/x.unity3d", Category: "AssetBundles/Android", MD5: "abcdef"})
	if err == nil {
		t.Fatal("全部主机失败时应报错")
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusServiceUnavailable {
		t.Fatalf("应返回最后一台的错误，得到 %v", err)
	}
	if a.hits != 1 || b.hits != 1 {
		t.Errorf("每台都该试一次：a=%d b=%d", a.hits, b.hits)
	}
}

// 清单解析以整棵树为单位换主机：同一逻辑 url 会在多个子清单里重复出现、以最后一条为准，
// 半棵来自 A 半棵来自 B 时那个"谁最后"就跨了主机，取出来的可能是任何一条。
func TestResolveFailsOverWholeTree(t *testing.T) {
	down := newHostSpy(t, http.StatusServiceUnavailable, "")
	up := newHostSpy(t, http.StatusOK, "")

	if _, err := sourceOver(t, down, up).Resolve(context.Background(), 202607290900); err != nil {
		t.Fatalf("次选主机可用时不应失败：%v", err)
	}
	if down.hits != 1 {
		t.Errorf("首台应只被试一次（整棵树重来，不是逐条接力）：%d", down.hits)
	}
	if up.hits == 0 {
		t.Error("应改用次选主机重新解析整棵树")
	}
}

// ctx 已取消时不该继续换主机空转。
func TestNoFailoverAfterCancel(t *testing.T) {
	a := newHostSpy(t, http.StatusOK, "")
	b := newHostSpy(t, http.StatusOK, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sourceOver(t, a, b).Download(ctx,
		&Content{URL: "a/x.unity3d", Category: "AssetBundles/Android", MD5: "abcdef"})
	if err == nil {
		t.Fatal("已取消的 ctx 应直接失败")
	}
	if b.hits != 0 {
		t.Errorf("取消后不该再试下一台，次选被请求了 %d 次", b.hits)
	}
}

// 下发缺失时不能把兜底也清掉——空列表应保留构造时的取值。
func TestWithResListIgnoresEmptyAndNil(t *testing.T) {
	s := NewSource(WithResList(nil))
	if len(s.res) != 1 || s.res[0].String() != urlx.MustParseBase(DefaultRes).String() {
		t.Errorf("空列表应保留内置默认 CDN，得到 %v", s.res)
	}
	s = NewSource(WithResList([]*url.URL{nil, nil}))
	if len(s.res) != 1 {
		t.Errorf("全 nil 的列表应被忽略，得到 %v", s.res)
	}
}
