package client

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// Folder 签名里的 req 必须真的从中间件穿透到折叠器。
//
// 这条单独测，是因为现有折叠器【全部】忽略 req：链路要是没接通，得等到第一个真正读它的
// 折叠器落地才会暴露（story/viewing 那类靠 request.story_id 定位改了哪条状态的端点）。
// 那时排查的是「新折叠器为什么不生效」，而根因在这条早就断掉的线上。
func TestFoldingMiddlewarePassesRequest(t *testing.T) {
	var got protocol.Request
	called := false

	reg := gamestate.NewRegistry()
	reg.Register((*fakeResp)(nil), func(_ *gamestate.PlayerState, req protocol.Request, _ any) {
		called, got = true, req
	})

	want := &fakeReq{}
	h := foldingMiddleware(gamestate.New(), reg)(
		func(context.Context, protocol.Request, any) (protocol.ResponseHeader, error) {
			return protocol.ResponseHeader{}, nil
		})
	if _, err := h(context.Background(), want, &fakeResp{}); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Fatal("折叠器未被调用")
	}
	if got != want {
		t.Fatalf("折叠器收到的 req = %#v，want %#v（中间件没把请求传下去）", got, want)
	}
}
