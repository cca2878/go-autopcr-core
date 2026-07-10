// Package race 是「角色赛马」域的游戏 API 能力面（chara_fortune/draw；对应 ref chara_fortune）。
//
// 是否处于赛马开放时段由母数据判定、今日是否已赛马由玩家状态（cf）判定，均在上层模块完成；
// 本域只负责按传入的 fortune_id/unit_id 发抽取请求。
package race

import (
	"context"

	racepb "github.com/cca2878/go-autopcr/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr/internal/client/internal/transport"
)

// API 是赛马域能力面契约（随功能在本包内累加）。
type API interface {
	// DrawCharaFortune 抽取今日赛马，返回首项奖励实得数量（宝石）。
	DrawCharaFortune(ctx context.Context, fortuneID, unitID int) (int, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造赛马域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) DrawCharaFortune(ctx context.Context, fortuneID, unitID int) (int, error) {
	resp, err := transport.Call[racepb.CharaFortuneDrawResponse](ctx, a.tr, &racepb.CharaFortuneDrawRequest{FortuneID: fortuneID, UnitID: unitID})
	if err != nil {
		return 0, err
	}
	if len(resp.RewardList) == 0 {
		return 0, nil
	}
	return resp.RewardList[0].Received, nil
}
