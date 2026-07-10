// Package room 是「家园」域的 DTO（家园产物收取、家具升级等；对应 ref room.py）。
package room

import (
	"net/url"

	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
)

var (
	urlRoomStart      = protocol.MustRelURL("room/start")
	urlRoomReceiveAll = protocol.MustRelURL("room/receive_all")
)

// RoomStartRequest 进入家园并拉取家园状态（含各家具的待收产物计数）。
type RoomStartRequest struct {
	protocol.RequestBase
	WacAutoOptionFlag int `msgpack:"wac_auto_option_flag" json:"wac_auto_option_flag"`
}

func (*RoomStartRequest) URL() *url.URL { return urlRoomStart }

// RoomUserItem 是家园中一件家具/装置（此处仅取判断待收所需字段）。
// item_count 为该家具当前累积、可收取的产物数量。
type RoomUserItem struct {
	SerialID      int `msgpack:"serial_id" json:"serial_id"`
	RoomItemID    int `msgpack:"room_item_id" json:"room_item_id"`
	RoomItemLevel int `msgpack:"room_item_level" json:"room_item_level"`
	ItemCount     int `msgpack:"item_count" json:"item_count"`
}

// RoomStartResponse 携带家园家具列表（其余字段由解码器忽略）。
type RoomStartResponse struct {
	protocol.ResponseBase
	UserRoomItemList []RoomUserItem `msgpack:"user_room_item_list" json:"user_room_item_list"`
}

// RoomReceiveItemAllRequest 一键收取家园全部待收产物（mana/体力等）。
type RoomReceiveItemAllRequest struct {
	protocol.RequestBase
}

func (*RoomReceiveItemAllRequest) URL() *url.URL { return urlRoomReceiveAll }

// InventoryInfo 是一件收取到的产物/奖励（type 为 eInventoryType 枚举值）。
type InventoryInfo struct {
	ID    int `msgpack:"id" json:"id"`
	Type  int `msgpack:"type" json:"type"`
	Count int `msgpack:"count" json:"count"`
}

// RoomReceiveItemAllResponse 携带本次收取到的产物列表（其余字段由解码器忽略）。
type RoomReceiveItemAllResponse struct {
	protocol.ResponseBase
	RewardList []InventoryInfo `msgpack:"reward_list" json:"reward_list"`
}
