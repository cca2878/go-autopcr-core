package transport

import (
	"context"
	"errors"
	"reflect"

	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// ZeroResponse 清零响应载体。任何"重发同一个 out"的重试路径都必须先调用它：解码器只覆盖本次
// 响应里出现的字段，上一轮残留的 server_error 会让重发后的'成功'响应被再次误判为业务错误。
func ZeroResponse(out any) {
	if out == nil {
		return
	}
	if v := reflect.ValueOf(out); v.Kind() == reflect.Pointer && !v.IsNil() {
		v.Elem().SetZero()
	}
}

// DefaultRetries 是网络错误的默认重试次数（复刻原 errorhandler）。
const DefaultRetries = 5

// ErrorHandler 复刻参考项目 misc.py 的 errorhandler：
//   - 网络错误重试至多 retries 次；
//   - 业务错误若含"维护"则升级为 PanicError；
//   - 其余错误直接上抛。
func ErrorHandler(retries int) Middleware {
	if retries < 0 {
		retries = DefaultRetries
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req protocol.Request, out any) (protocol.ResponseHeader, error) {
			left := retries
			for {
				header, err := next(ctx, req, out)
				if err == nil {
					return header, nil
				}
				var netErr *gameerr.NetworkError
				if errors.As(err, &netErr) {
					// 调用方已放弃：ctx 取消/超时也会被包成网络错误，重试只是对着死 ctx 空转。
					if ctx.Err() != nil {
						return header, err
					}
					if left <= 0 {
						return header, err
					}
					left--
					// 解码失败同样归类为网络错误，此时 out 可能已被写入半截字段。
					ZeroResponse(out)
					continue
				}
				// 兜底：内层已按同一判据升级过（见 Client.transport），此处覆盖未经它的路径。
				var apiErr *gameerr.APIError
				if errors.As(err, &apiErr) && gameerr.IsFatalBusiness(apiErr.ResultCode, apiErr.Message) {
					return header, gameerr.Panic("%s", apiErr.Message)
				}
				return header, err
			}
		}
	}
}
