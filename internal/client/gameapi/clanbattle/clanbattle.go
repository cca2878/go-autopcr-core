// Package clanbattle 是"公会战"域的游戏 API 能力面（clan_battle/top；对应参考项目 clan_battle_knive）。
//
// 是否已加入公会由上层据玩家状态判定（未加入时 clan_battle/top 会触发业务错误，故须先查后动）。
package clanbattle

import (
	"context"

	clanbattlepb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clanbattle"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Top 是公会战刀数摘要。
type Top struct {
	RemainingCount int   // 剩余整刀数
	Point          int   // 体力点数（满 900）
	CarryOverTimes []int // 尾刀补偿秒数列表（>0 者）
}

// API 是公会战域能力面契约（随功能在本包内累加）。
type API interface {
	// Top 返回指定公会的公会战刀数信息。
	Top(ctx context.Context, clanID int64) (Top, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造公会战域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Top(ctx context.Context, clanID int64) (Top, error) {
	resp, err := transport.Call[clanbattlepb.TopResponse](ctx, a.tr, &clanbattlepb.TopRequest{ClanID: clanID, IsFirst: 1})
	if err != nil {
		return Top{}, err
	}
	top := Top{RemainingCount: resp.RemainingCount, Point: resp.Point}
	for _, c := range resp.CarryOver {
		if c.Time > 0 {
			top.CarryOverTimes = append(top.CarryOverTimes, c.Time)
		}
	}
	return top, nil
}
