// Package clanbattle 是「公会战」域的 DTO（clan_battle/top；对应 ref clan_battle_knive）。
package clanbattle

import (
	"net/url"

	"github.com/cca2878/go-autopcr/internal/client/internal/protocol"
)

var urlTop = protocol.MustRelURL("clan_battle/top")

// CarryOverInfo 是一条尾刀（补偿时间）记录（此处仅取补偿秒数）。
type CarryOverInfo struct {
	Time int `msgpack:"time" json:"time"`
}

// TopRequest 拉取公会战首页（剩余刀数、体力点数、尾刀等）。
type TopRequest struct {
	protocol.RequestBase
	ClanID                int64 `msgpack:"clan_id" json:"clan_id"`
	IsFirst               int   `msgpack:"is_first" json:"is_first"`
	CurrentClanBattleCoin int   `msgpack:"current_clan_battle_coin" json:"current_clan_battle_coin"`
}

func (*TopRequest) URL() *url.URL { return urlTop }

// TopResponse 携带公会战刀数信息（其余字段由解码器忽略）。
type TopResponse struct {
	protocol.ResponseBase
	RemainingCount int             `msgpack:"remaining_count" json:"remaining_count"`
	Point          int             `msgpack:"point" json:"point"`
	CarryOver      []CarryOverInfo `msgpack:"carry_over" json:"carry_over"`
}
