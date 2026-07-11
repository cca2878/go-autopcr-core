// Package race 是「角色赛马」域的 DTO（chara_fortune/draw；对应 ref chara_fortune）。
package race

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var urlCharaFortuneDraw = protocol.MustRelURL("chara_fortune/draw")

// InventoryInfo 是一件奖励（此处仅取 received＝实得数量）。
type InventoryInfo struct {
	Received int `msgpack:"received" json:"received"`
}

// CharaFortuneDrawRequest 抽取今日赛马结果（fortune_id/unit_id 取自登录折叠的 cf 状态）。
type CharaFortuneDrawRequest struct {
	protocol.RequestBase
	FortuneID int `msgpack:"fortune_id" json:"fortune_id"`
	UnitID    int `msgpack:"unit_id" json:"unit_id"`
}

func (*CharaFortuneDrawRequest) URL() *url.URL { return urlCharaFortuneDraw }

// CharaFortuneDrawResponse 携带赛马奖励列表（其余字段由解码器忽略）。
type CharaFortuneDrawResponse struct {
	protocol.ResponseBase
	RewardList []InventoryInfo `msgpack:"reward_list" json:"reward_list"`
}
