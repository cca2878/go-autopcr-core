// Package account 是「账号/首页」域的游戏 API 能力面（home/index、后续 profile 等）。
package account

import (
	"context"

	accountpb "github.com/cca2878/go-autopcr/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr/internal/client/internal/transport"
)

// API 是账号/首页域能力面契约。
type API interface {
	// RefreshIndex 重新拉取 load/index，刷新聚合玩家状态（结果经折叠中间件落入 client.Data()）。
	RefreshIndex(ctx context.Context) error
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造账号域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) RefreshIndex(ctx context.Context) error {
	_, err := transport.Call[accountpb.LoadIndexResponse](ctx, a.tr, &accountpb.LoadIndexRequest{Carrier: "OPPO"})
	return err
}
