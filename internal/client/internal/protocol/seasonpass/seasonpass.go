// Package seasonpass 是"女神祭（季卡）"域的 DTO（season_ticket_new_index/accept；对应参考项目 seasonpass）。
package seasonpass

import (
	"net/url"
	"slices"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlIndex         = protocol.MustRelURL("season_ticket_new/index")
	urlMissionAccept = protocol.MustRelURL("season_ticket_new/accept")
)

// MissionStatusEnableReceive 是"可领取"的任务状态值（对应参考项目 eMissionStatusType.EnableReceive）。
const MissionStatusEnableReceive = 1

// UserMission 是女神祭的一条任务进度（此处仅取领取判定所需字段）。
type UserMission struct {
	MissionID     int `msgpack:"mission_id" json:"mission_id"`
	MissionStatus int `msgpack:"mission_status" json:"mission_status"`
}

// IndexRequest 拉取指定女神祭的总览（任务状态、等级等）。
type IndexRequest struct {
	protocol.RequestBase
	SeasonID int `msgpack:"season_id" json:"season_id"`
}

func (*IndexRequest) URL() *url.URL { return urlIndex }

// IndexResponse 携带女神祭任务与等级（其余字段由解码器忽略）。
type IndexResponse struct {
	protocol.ResponseBase
	SeasonpassLevel int           `msgpack:"seasonpass_level" json:"seasonpass_level"`
	Missions        []UserMission `msgpack:"missions" json:"missions"`
}

// MissionAcceptRequest 领取女神祭任务奖励（mission_id=0 表示一键领取全部可领）。
type MissionAcceptRequest struct {
	protocol.RequestBase
	SeasonID  int `msgpack:"season_id" json:"season_id"`
	MissionID int `msgpack:"mission_id" json:"mission_id"`
}

func (*MissionAcceptRequest) URL() *url.URL { return urlMissionAccept }

// MissionAcceptResponse 携带领取到的奖励与更新后的等级（其余字段由解码器忽略）。
type MissionAcceptResponse struct {
	protocol.ResponseBase
	SeasonpassLevel int                      `msgpack:"seasonpass_level" json:"seasonpass_level"`
	Rewards         []protocol.InventoryInfo `msgpack:"rewards" json:"rewards"`
}

// InventoryChanges 实现 protocol.RewardCarrier，'逆序'返回。
//
// 逆序照搬参考项目：它对应的 handler 写的是 `for reward in self.rewards[::-1]`
// （handlers.py:900），而这是参考项目全部 138 个折叠 handler 里'唯一'一处逆序遍历——只此一
// 家，说明不是笔误而是这个端点的特性。一次领取会横跨多个等级，逐条的 stock 若是累进中间值，
// 正序遍历最后留下的就会是最旧的那条。core 没有该端点的真机样本，故保持与参考项目一致。
func (r *MissionAcceptResponse) InventoryChanges() []protocol.InventoryInfo {
	out := slices.Clone(r.Rewards)
	slices.Reverse(out)
	return out
}
