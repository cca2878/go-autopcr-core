package daily

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlMissionIndex  = protocol.MustRelURL("mission/index")
	urlMissionAccept = protocol.MustRelURL("mission/accept")
)

// MissionRequestFlag 是 mission/index 的请求标志（quest_clear_rank=0 复刻权威客户端）。
type MissionRequestFlag struct {
	QuestClearRank int `msgpack:"quest_clear_rank" json:"quest_clear_rank"`
}

// MissionIndexRequest 拉取任务列表（含各任务状态）。
type MissionIndexRequest struct {
	protocol.RequestBase
	RequestFlag MissionRequestFlag `msgpack:"request_flag" json:"request_flag"`
}

func (*MissionIndexRequest) URL() *url.URL { return urlMissionIndex }

// UserMissionInfo 是一条任务（此处仅取归类与状态所需字段）。
// mission_status 取 eMissionStatusType：1=可领取(EnableReceive)、2=已领取。
type UserMissionInfo struct {
	MissionID     int `msgpack:"mission_id" json:"mission_id"`
	MissionStatus int `msgpack:"mission_status" json:"mission_status"`
}

// UserSeasonPackInfo 是一条月卡（季票）附带任务：received=0 即该档奖励尚未领取。
// 它与 missions 平行下发，同样按 type=1/2/4 归类领取，漏读会导致付费奖励永远收不到。
type UserSeasonPackInfo struct {
	MissionID int `msgpack:"mission_id" json:"mission_id"`
	Received  int `msgpack:"received" json:"received"`
}

// MissionIndexResponse 携带任务列表与月卡附带任务（其余字段由解码器忽略）。
type MissionIndexResponse struct {
	protocol.ResponseBase
	Missions   []UserMissionInfo    `msgpack:"missions" json:"missions"`
	SeasonPack []UserSeasonPackInfo `msgpack:"season_pack" json:"season_pack"`
}

// MissionAcceptRequest 领取某一类别（type=1/2/4）下全部可领取任务的奖励。
// id=0 / buy_id=0 表示领取该类别全部（复刻权威客户端）。
type MissionAcceptRequest struct {
	protocol.RequestBase
	Type  int `msgpack:"type" json:"type"`
	ID    int `msgpack:"id" json:"id"`
	BuyID int `msgpack:"buy_id" json:"buy_id"`
}

func (*MissionAcceptRequest) URL() *url.URL { return urlMissionAccept }

// MissionAcceptResponse 携带本次领取到的奖励列表（其余字段由解码器忽略）。
type MissionAcceptResponse struct {
	StaminaInfo *protocol.UserStaminaInfo `msgpack:"stamina_info" json:"stamina_info"`
	TeamLevel   int                       `msgpack:"team_level" json:"team_level"`
	protocol.ResponseBase
	Rewards []protocol.InventoryInfo `msgpack:"rewards" json:"rewards"`
}

// InventoryChanges 实现 protocol.RewardCarrier。core 无该端点的真机样本，做法照搬 ref 的
// MissionAcceptResponse（handlers.py:464，rewards 逐条走 update_inventory）。
func (r *MissionAcceptResponse) InventoryChanges() []protocol.InventoryInfo { return r.Rewards }

// StaminaSnapshot 实现 protocol.StaminaCarrier（对应 ref handlers.py:469）。
func (r *MissionAcceptResponse) StaminaSnapshot() *protocol.UserStaminaInfo { return r.StaminaInfo }
