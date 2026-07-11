package transport

import (
	"context"
	"errors"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// DefaultRetries 是网络错误的默认重试次数（复刻原 errorhandler）。
const DefaultRetries = 5

// ErrorHandler 复刻原项目 misc.py 的 errorhandler：
//   - 网络错误重试至多 retries 次；
//   - 业务错误若含「维护」则升级为 PanicError；
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
					if left <= 0 {
						return header, err
					}
					left--
					continue
				}
				var apiErr *gameerr.APIError
				if errors.As(err, &apiErr) && strings.Contains(apiErr.Message, "维护") {
					return header, gameerr.Panic("%s", apiErr.Message)
				}
				return header, err
			}
		}
	}
}
