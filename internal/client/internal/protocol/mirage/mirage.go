// Package mirage 是「追忆战」域的 DTO（mirage/top、mirage/receive_reward；对应 ref mirage_floor_receive）。
package mirage

import (
	"net/url"

	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
)

var (
	urlTop           = protocol.MustRelURL("mirage/top")
	urlReceiveReward = protocol.MustRelURL("mirage/receive_reward")
)

// InventoryInfo 是一件奖励（此处仅计数）。
type InventoryInfo struct {
	ID int `msgpack:"id" json:"id"`
}

// TopRequest 拉取追忆战首页（礼物池累积状态等）。
type TopRequest struct {
	protocol.RequestBase
}

func (*TopRequest) URL() *url.URL { return urlTop }

// TopResponse 携带追忆战礼物池信息。reward_full_time＝礼物池将满的时间戳（-1＝未通关）。
type TopResponse struct {
	protocol.ResponseBase
	RewardFullTime int64 `msgpack:"reward_full_time" json:"reward_full_time"`
}

// ReceiveRewardRequest 领取追忆战礼物池奖励。from_system_id 指定来源系统（追忆战＝135）。
type ReceiveRewardRequest struct {
	protocol.RequestBase
	FromSystemID int `msgpack:"from_system_id" json:"from_system_id"`
}

func (*ReceiveRewardRequest) URL() *url.URL { return urlReceiveReward }

// ReceiveRewardResponse 携带领取到的奖励（其余字段由解码器忽略）。
type ReceiveRewardResponse struct {
	protocol.ResponseBase
	RewardInfo []InventoryInfo `msgpack:"reward_info" json:"reward_info"`
}
