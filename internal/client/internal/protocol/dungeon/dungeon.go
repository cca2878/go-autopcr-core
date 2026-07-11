// Package dungeon 是「地下城」域的 DTO（dungeon/info；对应 ref underground 扫荡）。
package dungeon

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var urlDungeonInfo = protocol.MustRelURL("dungeon/info")

// DungeonInfoRequest 拉取地下城信息（当前所在区域、剩余挑战次数等）。
type DungeonInfoRequest struct {
	protocol.RequestBase
}

func (*DungeonInfoRequest) URL() *url.URL { return urlDungeonInfo }

// RestChallengeInfo 是某类地下城的剩余/上限挑战次数。
type RestChallengeInfo struct {
	DungeonType int `msgpack:"dungeon_type" json:"dungeon_type"`
	Count       int `msgpack:"count" json:"count"`
	MaxCount    int `msgpack:"max_count" json:"max_count"`
}

// DungeonInfoResponse 携带地下城信息（其余字段由解码器忽略）。enter_area_id＝当前所在区域（0＝未进入）。
type DungeonInfoResponse struct {
	protocol.ResponseBase
	EnterAreaID        int                 `msgpack:"enter_area_id" json:"enter_area_id"`
	RestChallengeCount []RestChallengeInfo `msgpack:"rest_challenge_count" json:"rest_challenge_count"`
}
