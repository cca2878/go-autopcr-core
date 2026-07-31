package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

func TestParseStoreURLVersion(t *testing.T) {
	cases := []struct {
		name     string
		storeURL string
		want     string
		wantOK   bool
	}{
		{
			name:     "真实样例",
			storeURL: "https://pkg.biligame.com/games/gzlj_11.7.2_20260715_154600_b8233_896629.apk",
			want:     "11.7.2",
			wantOK:   true,
		},
		{name: "空字符串", storeURL: "", wantOK: false},
		{name: "不含版本号形状", storeURL: "https://example.com/other.apk", wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseStoreURLVersion(c.storeURL)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Errorf("version = %q, want %q", got, c.want)
			}
		})
	}
}

// versionProbeResponse 内嵌 ResponseBase（实现 ErrorCarrier），使解码路径与真实响应一致。
type versionProbeResponse struct {
	protocol.ResponseBase
	OK bool `json:"ok"`
}

// versionProbeRequest 走明文 JSON 通道，无需构造加密载荷。
type versionProbeRequest struct{ protocol.RequestBase }

func (r *versionProbeRequest) URL() *url.URL { return protocol.MustRelURL("test/version-probe") }
func (r *versionProbeRequest) Crypted() bool { return false }

// 服务端在 APP-VER 过期时拒绝请求（result_code=204、status=3）并在 store_url 里下发真实版本号；
// 这与「会话失效」共用同一个 status（见 relogin.go 的 statusSessionInvalid），若不在传输层就地
// 纠正，会被误判成要重登。本用例钉住：探测到 store_url 后自动升级 APP-VER 头并重发一次，
// 最终把【第二次】的成功响应当作本次调用的结果返回，且过程中只多打一次请求。
func TestTransportSelfHealsStaleAppVer(t *testing.T) {
	const staleVer = "11.4.0"
	const realVer = "11.7.2"

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			if got := r.Header.Get("APP-VER"); got != staleVer {
				t.Errorf("第一次请求 APP-VER = %q, want %q", got, staleVer)
			}
			_, _ = fmt.Fprintf(w, `{"data_headers":{"result_code":204,"status":3,`+
				`"store_url":"https://pkg.biligame.com/games/gzlj_%s_20260715_154600_b8233_896629.apk"},`+
				`"data":{"server_error":{"title":"","message":"发生了错误。\n回到标题界面。","status":3}}}`, realVer)
		case 2:
			if got := r.Header.Get("APP-VER"); got != realVer {
				t.Errorf("重试请求 APP-VER = %q, want %q（应已自动纠正）", got, realVer)
			}
			_, _ = fmt.Fprint(w, `{"data_headers":{"result_code":1},"data":{"ok":true}}`)
		default:
			t.Fatalf("不应发出第 %d 次请求", calls)
		}
	}))
	defer srv.Close()

	c := New(&fakeCred{apiRoot: srv.URL}, WithHTTPClient(srv.Client()))
	c.SetHeader("APP-VER", staleVer)

	var out versionProbeResponse
	header, err := c.transport(context.Background(), &versionProbeRequest{}, &out)
	if err != nil {
		t.Fatalf("自愈后不应再报错，got %v", err)
	}
	if calls != 2 {
		t.Fatalf("调用次数 = %d, want 2（首次失败 + 自愈重试各一次）", calls)
	}
	if header.ResultCode != 1 {
		t.Errorf("最终 ResultCode = %d, want 1", header.ResultCode)
	}
	if !out.OK {
		t.Error("最终响应应解出 data.ok=true")
	}
	if out.ServerError != nil {
		t.Errorf("最终响应不应残留第一次的 server_error，got %+v", out.ServerError)
	}

	c.mu.Lock()
	got := c.headers["APP-VER"]
	c.mu.Unlock()
	if got != realVer {
		t.Errorf("Client 的 APP-VER 头 = %q, want %q（应已持久更新，供后续请求复用）", got, realVer)
	}
}

// store_url 里的版本号与当前 APP-VER 相同时不该多打一次请求——常态下每次维护状态调用都可能
// 带 store_url，若不做「有变化才重试」的判断，会让本无问题的调用平白多一次往返。
func TestTransportNoRetryWhenVersionUnchanged(t *testing.T) {
	const ver = "11.7.2"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, `{"data_headers":{"result_code":1,`+
			`"store_url":"https://pkg.biligame.com/games/gzlj_%s_20260715_154600_b8233_896629.apk"},`+
			`"data":{"ok":true}}`, ver)
	}))
	defer srv.Close()

	c := New(&fakeCred{apiRoot: srv.URL}, WithHTTPClient(srv.Client()))
	c.SetHeader("APP-VER", ver)

	var out versionProbeResponse
	if _, err := c.transport(context.Background(), &versionProbeRequest{}, &out); err != nil {
		t.Fatalf("不应报错，got %v", err)
	}
	if calls != 1 {
		t.Errorf("调用次数 = %d, want 1（版本未变不应重试）", calls)
	}
}
