// Package emblem 是「称号」域的游戏 API 能力面（emblem/top；对应 ref missing_emblem）。
package emblem

import (
	"context"

	emblempb "github.com/cca2878/go-autopcr/internal/client/internal/protocol/emblem"
	"github.com/cca2878/go-autopcr/internal/client/internal/transport"
)

// API 是称号域能力面契约（随功能在本包内累加）。
type API interface {
	// OwnedEmblemIDs 返回玩家已拥有的称号 id 集合（供缺口判定）。
	OwnedEmblemIDs(ctx context.Context) (map[int]struct{}, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造称号域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) OwnedEmblemIDs(ctx context.Context) (map[int]struct{}, error) {
	resp, err := transport.Call[emblempb.EmblemTopResponse](ctx, a.tr, &emblempb.EmblemTopRequest{})
	if err != nil {
		return nil, err
	}
	owned := make(map[int]struct{}, len(resp.UserEmblemList))
	for _, e := range resp.UserEmblemList {
		owned[e.EmblemID] = struct{}{}
	}
	return owned, nil
}
