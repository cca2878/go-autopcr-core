package client

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"

	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// 会话失效的判定与重登策略（对应原项目 clientpool.SessionErrorHandler + sessionmgr.request）。
//
// 游戏服在若干「严重」错误下会丢弃会话（被其他客户端顶号、客户端数据与服务端对不上等），
// 要求客户端重走登录流程。这类错误与模块逻辑无关，故由传输层自愈，不该冒泡成模块失败。
// 注意重登只是【用同一 access_key 重跑游戏登录序列】——重新获取 access_key 是外壳的事。
const (
	// resultCodeKicked 是被其他客户端顶号（用户确认）。
	resultCodeKicked = 6002
	// resultCodeSessionLost 是原项目与 6002 并列处理的另一个会话错误码；上游未命名，照搬。
	resultCodeSessionLost = 4
	// resultCodeBadCredential 是假凭据 / 未知账号的正常返回。它的 message【同样】含
	// titleScreenMarker，但同一 access_key 重跑登录必然再次失败，故显式豁免：错误照样
	// 上抛，只是不做无用的重登（见 M1 线缆记录：probe 用假 access_key 即得 107）。
	resultCodeBadCredential = 107

	// statusSessionInvalid 是 server_error.status 中表示会话失效的取值（原项目 ex.status == 3）。
	statusSessionInvalid = 3

	// titleScreenMarker 是「需要回到标题界面」类消息的特征串。服务端要求重登时消息里
	// 一定含它（用户确认），但反过来不成立（见 resultCodeBadCredential），故它是必要
	// 条件而非充分条件——用作未知错误码的兜底判据。
	titleScreenMarker = "回到标题界面"

	// maxReloginRetries 是「重登后原样重发」的次数上限（原项目 SESSION_ERROR_MAX_RETRY）。
	maxReloginRetries = 2
)

// sessionFault 描述一次失败对会话的影响。
type sessionFault int

const (
	faultNone  sessionFault = iota // 与会话无关
	faultStale                     // 会话已失效，但本次请求不重发
	faultRetry                     // 会话已失效，重登后原样重发
)

// classifySession 判定错误是否意味着会话已失效，以及能否重发。
//
// 可重发仅限【已知的会话错误码】：服务端在这两个码下必定没有执行请求，重发是安全的。
// 靠 message 兜底命中的未知码不重发——无从判断服务端是否已部分执行，盲目重发可能重复扣资源。
func classifySession(err error) (*gameerr.APIError, sessionFault) {
	var api *gameerr.APIError
	if !errors.As(err, &api) {
		return nil, faultNone
	}
	if api.ResultCode == resultCodeBadCredential {
		return api, faultNone
	}
	switch api.ResultCode {
	case resultCodeKicked, resultCodeSessionLost:
		return api, faultRetry
	}
	if api.Status == statusSessionInvalid || strings.Contains(api.Message, titleScreenMarker) {
		return api, faultStale
	}
	return api, faultNone
}

// reloginKey 标记「本请求属于重登序列自身」，使其绕过重登中间件（否则登录序列里的
// 请求一旦失败会再次触发重登，无限递归）。
type reloginKey struct{}

func markRelogin(ctx context.Context) context.Context {
	return context.WithValue(ctx, reloginKey{}, struct{}{})
}

func inRelogin(ctx context.Context) bool { return ctx.Value(reloginKey{}) != nil }

// sessionGuard 维护「会话是否已被服务端判定失效」这一位状态，并在下次请求前重走登录序列。
//
// 惰性重登（置位 + 请求前 ensure）复刻原项目；但【不重置玩家状态】——原项目换新 datamgr
// 是因为它的连接池会跨账号复用同一 wrapper，而本库一个 client 绑定一份凭据，重登时
// load/index + home/index 会把权威字段原样覆盖回来，清空反而会丢掉本轮模块已折叠的数据。
type sessionGuard struct {
	login  func(context.Context) error // 重登动作（注入以便单测）
	logger *slog.Logger

	mu    sync.Mutex
	stale bool
}

// invalidate 标记会话失效，下一次请求前会重登。
func (g *sessionGuard) invalidate() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stale = true
}

// markFresh 声明会话是新的（显式 Login 成功后调用），清掉待重登标记。
func (g *sessionGuard) markFresh() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stale = false
}

// ensure 在会话被判失效时重走登录序列。锁跨整个登录过程，使并发请求只重登一次。
func (g *sessionGuard) ensure(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.stale {
		return nil
	}
	g.logger.Warn("会话已失效，重新登录")
	if err := g.login(markRelogin(ctx)); err != nil {
		return err
	}
	g.stale = false
	return nil
}

// middleware 返回重登中间件。它必须是【最外层】中间件：网络重试与状态折叠都应发生在
// 它内部（重登时发出的登录请求本身要经过折叠中间件才能更新玩家状态）。
func (g *sessionGuard) middleware() transport.Middleware {
	return func(next transport.Handler) transport.Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			if inRelogin(ctx) {
				return next(ctx, req, out)
			}
			for attempt := 0; ; attempt++ {
				if err := g.ensure(ctx); err != nil {
					return protocol.ResponseHeader{}, err
				}
				header, err := next(ctx, req, out)
				if err == nil {
					return header, nil
				}
				api, fault := classifySession(err)
				if fault == faultNone {
					return header, err
				}
				g.invalidate()
				if fault != faultRetry || attempt >= maxReloginRetries {
					return header, err
				}
				// 上一轮已把 server_error 解进 out，而解码器不会清除本次响应里缺席的
				// 字段：残留的 server_error 会让重发后的【成功】响应被再次误判为业务错误。
				zeroResponse(out)
				g.logger.Warn("会话错误，重登后重发请求",
					"url", req.URL(), "result_code", api.ResultCode, "attempt", attempt+1)
			}
		}
	}
}

// zeroResponse 清零响应载体（见调用处：重发前必须抹掉上一轮的 server_error 残留）。
func zeroResponse(out any) {
	if out == nil {
		return
	}
	if v := reflect.ValueOf(out); v.Kind() == reflect.Pointer && !v.IsNil() {
		v.Elem().SetZero()
	}
}
