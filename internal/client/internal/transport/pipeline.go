package transport

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// Handler 处理一次请求：把 req 发出，将响应数据解码进 out（*R），返回响应头。
//
// out 为 any 而非泛型，使中间件保持非泛型（多数中间件无需知道具体响应类型）；
// 泛型仅在 Call 门面处出现。
type Handler func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error)

// Middleware 包裹一个 Handler，返回新的 Handler。
type Middleware func(next Handler) Handler

// Chain 将多个中间件组合为一个：Chain(a, b, c)(h) == a(b(c(h)))。
// 即靠前的中间件在外层。
func Chain(mws ...Middleware) Middleware {
	return func(final Handler) Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}
