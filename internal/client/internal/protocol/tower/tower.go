// Package tower 是"露娜塔"域的 DTO（tower/top；对应参考项目 tower_cloister_sweep）。
package tower

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var urlTop = protocol.MustRelURL("tower/top")

// TopRequest 拉取露娜塔首页（回廊扫荡次数、通关状态等）。
type TopRequest struct {
	protocol.RequestBase
	IsFirst              int `msgpack:"is_first" json:"is_first"`
	ReturnClearedExQuest int `msgpack:"return_cleared_ex_quest" json:"return_cleared_ex_quest"`
}

func (*TopRequest) URL() *url.URL { return urlTop }

// TopResponse 携带露娜塔回廊状态（其余字段由解码器忽略）。
type TopResponse struct {
	protocol.ResponseBase
	CloisterRemainClearCount int `msgpack:"cloister_remain_clear_count" json:"cloister_remain_clear_count"`
	CloisterFirstClearedFlag int `msgpack:"cloister_first_cleared_flag" json:"cloister_first_cleared_flag"`
}
