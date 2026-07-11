// Package seasonpass 是「女神祭（季卡）」域的游戏 API 能力面（对应 ref seasonpass）。
//
// 是否有进行中的女神祭由母数据判定（见 masterdata/seasonpass），本域只按 season_id 发包。
package seasonpass

import (
	"context"

	seasonpasspb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/seasonpass"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Index 是女神祭总览摘要（供先查后动：判断是否有可领任务）。
type Index struct {
	Level             int  // 当前女神祭等级
	HasReceivableTask bool // 是否有「可领取」状态的任务
}

// API 是女神祭域能力面契约（随功能在本包内累加）。
type API interface {
	// Index 返回指定女神祭的总览（等级、是否有可领任务）。
	Index(ctx context.Context, seasonID int) (Index, error)
	// AcceptMissions 一键领取指定女神祭的全部可领任务奖励，返回领取到的奖励件数与更新后等级。
	AcceptMissions(ctx context.Context, seasonID int) (rewardCount, level int, err error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造女神祭域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Index(ctx context.Context, seasonID int) (Index, error) {
	resp, err := transport.Call[seasonpasspb.IndexResponse](ctx, a.tr, &seasonpasspb.IndexRequest{SeasonID: seasonID})
	if err != nil {
		return Index{}, err
	}
	idx := Index{Level: resp.SeasonpassLevel}
	for _, m := range resp.Missions {
		if m.MissionStatus == seasonpasspb.MissionStatusEnableReceive {
			idx.HasReceivableTask = true
			break
		}
	}
	return idx, nil
}

func (a *Impl) AcceptMissions(ctx context.Context, seasonID int) (int, int, error) {
	resp, err := transport.Call[seasonpasspb.MissionAcceptResponse](ctx, a.tr, &seasonpasspb.MissionAcceptRequest{SeasonID: seasonID, MissionID: 0})
	if err != nil {
		return 0, 0, err
	}
	return len(resp.Rewards), resp.SeasonpassLevel, nil
}
