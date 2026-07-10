// Package arena 是「竞技场」域的 DTO（jjc/pjjc 时间奖励领取；对应 ref jjc_reward）。
package arena

import (
	"net/url"

	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
)

var (
	urlArenaInfo        = protocol.MustRelURL("arena/info")
	urlArenaReward      = protocol.MustRelURL("arena/time_reward_accept")
	urlGrandArenaInfo   = protocol.MustRelURL("grand_arena/info")
	urlGrandArenaReward = protocol.MustRelURL("grand_arena/time_reward_accept")
)

// RewardInfo 是待领取的时间奖励信息（此处仅取 count＝当前可领数量）。
type RewardInfo struct {
	Count int `msgpack:"count" json:"count"`
}

// ArenaInfoRequest 拉取竞技场信息（含时间奖励可领数量）。
type ArenaInfoRequest struct {
	protocol.RequestBase
}

func (*ArenaInfoRequest) URL() *url.URL { return urlArenaInfo }

// ArenaInfoResponse 携带竞技场信息（其余字段由解码器忽略）。
type ArenaInfoResponse struct {
	protocol.ResponseBase
	RewardInfo *RewardInfo `msgpack:"reward_info" json:"reward_info"`
}

// ArenaTimeRewardAcceptRequest 领取竞技场时间奖励（jjc 币）。
type ArenaTimeRewardAcceptRequest struct {
	protocol.RequestBase
}

func (*ArenaTimeRewardAcceptRequest) URL() *url.URL { return urlArenaReward }

// ArenaTimeRewardAcceptResponse 为领取结果（无需读取的字段由解码器忽略）。
type ArenaTimeRewardAcceptResponse struct {
	protocol.ResponseBase
}

// GrandArenaInfoRequest 拉取公主竞技场信息（含时间奖励可领数量）。
type GrandArenaInfoRequest struct {
	protocol.RequestBase
}

func (*GrandArenaInfoRequest) URL() *url.URL { return urlGrandArenaInfo }

// GrandArenaInfoResponse 携带公主竞技场信息（其余字段由解码器忽略）。
type GrandArenaInfoResponse struct {
	protocol.ResponseBase
	RewardInfo *RewardInfo `msgpack:"reward_info" json:"reward_info"`
}

// GrandArenaTimeRewardAcceptRequest 领取公主竞技场时间奖励（pjjc 币）。
type GrandArenaTimeRewardAcceptRequest struct {
	protocol.RequestBase
}

func (*GrandArenaTimeRewardAcceptRequest) URL() *url.URL { return urlGrandArenaReward }

// GrandArenaTimeRewardAcceptResponse 为领取结果（无需读取的字段由解码器忽略）。
type GrandArenaTimeRewardAcceptResponse struct {
	protocol.ResponseBase
}
