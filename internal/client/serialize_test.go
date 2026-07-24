package client

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// 并发调用必须互斥地穿过「串行化 → 折叠」，且不与重登互锁。
// 用 -race 跑才有意义：折叠写的是共享 PlayerState。
func TestSerializeMiddleware_ConcurrentCallsAreExclusive(t *testing.T) {
	var mu sync.Mutex
	var inFlight, maxInFlight int

	var callMu sync.Mutex
	h := serializeMiddleware(&callMu)(func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		mu.Lock()
		inFlight--
		mu.Unlock()
		return protocol.ResponseHeader{}, nil
	})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := h(context.Background(), &fakeReq{}, &fakeResp{}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if maxInFlight != 1 {
		t.Fatalf("并发穿透了串行化：最大同时在飞 %d", maxInFlight)
	}
}

// 重登的登录请求会再次穿过本链；串行化必须在守卫【之内】，否则同一把非重入锁会自死锁。
// 本测试若挂住即回归（go test 超时会报 panic 栈）。
func TestSerializeMiddleware_ReloginDoesNotDeadlock(t *testing.T) {
	var callMu sync.Mutex
	var chain transport.Handler

	g := &sessionGuard{logger: slog.New(slog.DiscardHandler)}
	// 重登动作：模拟登录序列——经同一条链再发一个请求。
	g.login = func(ctx context.Context) error {
		_, err := chain(markRelogin(ctx), &fakeReq{}, &fakeResp{})
		return err
	}
	g.invalidate() // 让首个请求前必先重登

	inner := func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
		return protocol.ResponseHeader{}, nil
	}
	chain = g.middleware()(serializeMiddleware(&callMu)(inner))

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := chain(context.Background(), &fakeReq{}, &fakeResp{}); err != nil {
			t.Error(err)
		}
	}()
	<-done // 死锁则此处永久阻塞，由 go test 的超时兜底报错
}
