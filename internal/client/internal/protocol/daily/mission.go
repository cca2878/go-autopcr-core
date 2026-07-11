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

// MissionIndexResponse 携带任务列表（其余字段由解码器忽略）。
type MissionIndexResponse struct {
	protocol.ResponseBase
	Missions []UserMissionInfo `msgpack:"missions" json:"missions"`
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
	protocol.ResponseBase
	Rewards []InventoryInfo `msgpack:"rewards" json:"rewards"`
}
