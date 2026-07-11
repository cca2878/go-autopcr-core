// Package mirage 是「追忆战」域的游戏 API 能力面（对应 ref mirage_floor_receive）。
//
// 是否解锁由上层据玩家任务状态判定；礼物池是否够领由母数据累积天数 + top 的 reward_full_time
// 共同判定（见模块）。本域只负责发包。
package mirage

import (
	"context"

	miragepb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/mirage"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// API 是追忆战域能力面契约（随功能在本包内累加）。
type API interface {
	// RewardFullTime 返回追忆战礼物池将满的时间戳（-1＝未通关追忆战）。
	RewardFullTime(ctx context.Context) (int64, error)
	// ReceiveReward 领取指定来源系统的追忆战礼物池奖励，返回领取到的件数。
	ReceiveReward(ctx context.Context, fromSystemID int) (int, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造追忆战域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) RewardFullTime(ctx context.Context) (int64, error) {
	resp, err := transport.Call[miragepb.TopResponse](ctx, a.tr, &miragepb.TopRequest{})
	if err != nil {
		return 0, err
	}
	return resp.RewardFullTime, nil
}

func (a *Impl) ReceiveReward(ctx context.Context, fromSystemID int) (int, error) {
	resp, err := transport.Call[miragepb.ReceiveRewardResponse](ctx, a.tr, &miragepb.ReceiveRewardRequest{FromSystemID: fromSystemID})
	if err != nil {
		return 0, err
	}
	return len(resp.RewardInfo), nil
}
