package daily

import (
	"context"

	dailypb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// missionStatusEnableReceive 是 eMissionStatusType.EnableReceive（可领取）。
const missionStatusEnableReceive = 1

// Mission 是一条任务及其是否可领取（供先查后领）。
type Mission struct {
	ID         int
	Receivable bool
}

// MissionList 拉取任务列表（mission/index），返回各任务及其是否可领取。月卡（季票）附带任务
// 与普通任务平行下发、走同一套 type 归类领取，故一并并入返回（对应 ref 对 season_pack 的或判）。
func (a *Impl) MissionList(ctx context.Context) ([]Mission, error) {
	resp, err := transport.Call[dailypb.MissionIndexResponse](ctx, a.tr, &dailypb.MissionIndexRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]Mission, 0, len(resp.Missions)+len(resp.SeasonPack))
	for _, m := range resp.Missions {
		out = append(out, Mission{ID: m.MissionID, Receivable: m.MissionStatus == missionStatusEnableReceive})
	}
	for _, sp := range resp.SeasonPack {
		out = append(out, Mission{ID: sp.MissionID, Receivable: sp.Received == 0})
	}
	return out, nil
}

// AcceptMissions 领取某一类别（category=1/2/4）下全部可领取任务的奖励。
// 调用方须先据母数据确认该类别确有可领取任务，避免对空类别触发业务错误。
func (a *Impl) AcceptMissions(ctx context.Context, category int) ([]Reward, error) {
	resp, err := transport.Call[dailypb.MissionAcceptResponse](ctx, a.tr, &dailypb.MissionAcceptRequest{Type: category})
	if err != nil {
		return nil, err
	}
	out := make([]Reward, len(resp.Rewards))
	for i, r := range resp.Rewards {
		out[i] = Reward{Type: r.Type, ID: r.ID, Count: r.Count}
	}
	return out, nil
}
