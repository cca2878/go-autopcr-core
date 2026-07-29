package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 解码失败被 ProtocolError 化之后，【必须仍然照旧参与网络重试】。
//
// 这条是本次错误结构化里唯一碰得到既有行为的地方：Client.transport 把 decodeEnvelope 的
// 失败包成 gameerr.Network(...)，ErrorHandler 据 errors.As(*NetworkError) 决定重发。若哪天
// 有人「顺手」把解码失败改成直接返回 ProtocolError，重试就会静默消失——这个用例会先红。
func TestErrorHandlerRetriesWrappedProtocolError(t *testing.T) {
	const retries = 3
	calls := 0
	decodeFailure := gameerr.Network(gameerr.Protocol("msgpack 信封", errors.New("坏字节")))

	h := ErrorHandler(retries)(func(context.Context, protocol.Request, any) (protocol.ResponseHeader, error) {
		calls++
		return protocol.ResponseHeader{}, decodeFailure
	})

	_, err := h(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("重试耗尽后应返回错误")
	}
	if want := retries + 1; calls != want {
		t.Errorf("调用次数 = %d, want %d（首次 + %d 次重试）", calls, want, retries)
	}

	// 分类看到的是最外层的判断（可重试），而具体成因仍可点名取出。
	if got := errs.Classify(err).Kind; got != errs.KindTransient {
		t.Errorf("Classify = %v, want KindTransient", got)
	}
	var pe *gameerr.ProtocolError
	if !errors.As(err, &pe) {
		t.Fatal("应能 errors.As 出底层的 ProtocolError")
	}
	if pe.Stage != "msgpack 信封" {
		t.Errorf("Stage = %q, want \"msgpack 信封\"", pe.Stage)
	}
}

// 业务错误不该被当成网络错误重试——与上一个用例互为对照，确认 As 判定没有放宽。
func TestErrorHandlerDoesNotRetryAPIError(t *testing.T) {
	calls := 0
	h := ErrorHandler(3)(func(context.Context, protocol.Request, any) (protocol.ResponseHeader, error) {
		calls++
		return protocol.ResponseHeader{}, &gameerr.APIError{Message: "礼物箱是空的", ResultCode: 1}
	})

	if _, err := h(context.Background(), nil, nil); err == nil {
		t.Fatal("业务错误应原样上抛")
	}
	if calls != 1 {
		t.Errorf("调用次数 = %d, want 1（业务错误不重试）", calls)
	}
}

// 上一个用例直接构造了 Network(Protocol(...))，钉的是 ErrorHandler 那一半；这个用例走真实
// 的 Client.transport，钉的是另一半：解码失败【确实被包成了 NetworkError】。少了它，有人把
// client.go 里那句 gameerr.Network(err) 改成直接返回 err，上面的用例照样绿，而重试已经没了。
func TestTransportWrapsDecodeFailureAsNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{这不是合法 JSON"))
	}))
	defer srv.Close()

	c := New(&fakeCred{apiRoot: srv.URL}, WithHTTPClient(srv.Client()))
	var out struct{}
	_, err := c.transport(context.Background(), &fakeRequest{}, &out)
	if err == nil {
		t.Fatal("响应体解不开时应返回错误")
	}

	var ne *gameerr.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("解码失败必须包成 NetworkError（否则 ErrorHandler 不会重试），得到 %T: %v", err, err)
	}
	var pe *gameerr.ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("NetworkError 里应包着 ProtocolError，得到 %v", err)
	}
	if got := errs.Classify(err).Kind; got != errs.KindTransient {
		t.Errorf("Classify = %v, want KindTransient", got)
	}
}

// fakeCred 是最小可用凭据：只为把 Client 装配起来，不参与任何鉴权。
type fakeCred struct{ apiRoot string }

func (c *fakeCred) Login(context.Context) (string, string, error) { return "1", "k", nil }
func (c *fakeCred) Header() map[string]string                     { return map[string]string{} }
func (c *fakeCred) APIRoot() string                               { return c.apiRoot }
func (c *fakeCred) PlatformID() string                            { return "2" }
func (c *fakeCred) ChannelID() string                             { return "1" }
func (c *fakeCred) DoCaptcha(context.Context) (*captcha.Result, error) {
	return nil, captcha.ErrNoSolver
}

// fakeRequest 走明文 JSON 通道（Crypted=false），使测试不必构造加密载荷。
type fakeRequest struct{ protocol.RequestBase }

func (r *fakeRequest) URL() *url.URL { return protocol.MustRelURL("test/endpoint") }
func (r *fakeRequest) Crypted() bool { return false }

// gameerr 全体都来自游戏服务这一域（那正是这个包的边界），处置类别则各不相同。
func TestGameErrClasses(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want errs.Kind
	}{
		{"网络", gameerr.Network(errors.New("超时")), errs.KindTransient},
		{"协议", gameerr.Protocol("data", errors.New("坏")), errs.KindCorrupt},
		{"业务", &gameerr.APIError{Message: "不行"}, errs.KindRejected},
		{"致命", gameerr.Panic("维护中"), errs.KindRejected},
		{"风控", gameerr.Risk(2, nil, nil), errs.KindRejected},
		{"会话断点", &gameerr.SessionBreakError{Cause: errors.New("顶号")}, errs.KindTransient},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := errs.Classify(c.err)
			if got.Kind != c.want {
				t.Errorf("Kind = %v, want %v", got.Kind, c.want)
			}
			if got.Domain != errs.DomainGameAPI {
				t.Errorf("Domain = %v, want DomainGameAPI", got.Domain)
			}
		})
	}
}
