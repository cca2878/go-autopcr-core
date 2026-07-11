// Package tower 是「露娜塔」域的游戏 API 能力面（tower/top；对应 ref tower_cloister_sweep）。
//
// 是否解锁由上层据玩家任务状态判定；是否开放期由母数据排程判定（见模块）。本域只负责发包。
package tower

import (
	"context"

	towerpb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/tower"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Top 是露娜塔回廊状态摘要。
type Top struct {
	CloisterRemainClear  int  // 回廊剩余可扫荡次数
	CloisterFirstCleared bool // 回廊首关是否已通关（未通关则不能扫荡）
}

// API 是露娜塔域能力面契约（随功能在本包内累加）。
type API interface {
	// Top 返回露娜塔回廊状态。
	Top(ctx context.Context) (Top, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造露娜塔域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Top(ctx context.Context) (Top, error) {
	resp, err := transport.Call[towerpb.TopResponse](ctx, a.tr, &towerpb.TopRequest{IsFirst: 1, ReturnClearedExQuest: 0})
	if err != nil {
		return Top{}, err
	}
	return Top{
		CloisterRemainClear:  resp.CloisterRemainClearCount,
		CloisterFirstCleared: resp.CloisterFirstClearedFlag == 1,
	}, nil
}
