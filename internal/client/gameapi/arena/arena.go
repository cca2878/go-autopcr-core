// Package arena 是「竞技场」域的游戏 API 能力面（jjc/pjjc 时间奖励领取；对应 ref jjc_reward）。
//
// 能力方法按具体系统分文件，共用同一 Impl（只持传输句柄）。解锁门禁（是否已通关对应任务）由
// 上层模块经玩家状态判定，本域只负责发包。
package arena

import (
	"context"

	arenapb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/arena"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// API 是竞技场域能力面契约（随功能在本包内累加）。
type API interface {
	// ArenaRewardCount 返回竞技场当前可领取的时间奖励（jjc 币）数量。
	ArenaRewardCount(ctx context.Context) (int, error)
	// ReceiveArenaReward 领取竞技场时间奖励。
	ReceiveArenaReward(ctx context.Context) error
	// GrandArenaRewardCount 返回公主竞技场当前可领取的时间奖励（pjjc 币）数量。
	GrandArenaRewardCount(ctx context.Context) (int, error)
	// ReceiveGrandArenaReward 领取公主竞技场时间奖励。
	ReceiveGrandArenaReward(ctx context.Context) error
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造竞技场域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) ArenaRewardCount(ctx context.Context) (int, error) {
	resp, err := transport.Call[arenapb.ArenaInfoResponse](ctx, a.tr, &arenapb.ArenaInfoRequest{})
	if err != nil {
		return 0, err
	}
	return rewardCount(resp.RewardInfo), nil
}

func (a *Impl) ReceiveArenaReward(ctx context.Context) error {
	_, err := transport.Call[arenapb.ArenaTimeRewardAcceptResponse](ctx, a.tr, &arenapb.ArenaTimeRewardAcceptRequest{})
	return err
}

func (a *Impl) GrandArenaRewardCount(ctx context.Context) (int, error) {
	resp, err := transport.Call[arenapb.GrandArenaInfoResponse](ctx, a.tr, &arenapb.GrandArenaInfoRequest{})
	if err != nil {
		return 0, err
	}
	return rewardCount(resp.RewardInfo), nil
}

func (a *Impl) ReceiveGrandArenaReward(ctx context.Context) error {
	_, err := transport.Call[arenapb.GrandArenaTimeRewardAcceptResponse](ctx, a.tr, &arenapb.GrandArenaTimeRewardAcceptRequest{})
	return err
}

func rewardCount(info *arenapb.RewardInfo) int {
	if info == nil {
		return 0
	}
	return info.Count
}
