// Package daily 是"每日收取"域的 DTO（礼物箱、任务奖励等；对应参考项目 daily.py）。
package daily

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlPresentIndex      = protocol.MustRelURL("present/index")
	urlPresentReceiveAll = protocol.MustRelURL("present/receive_all")
)

// PresentIndexRequest 拉取礼物箱内容（全部、不限时、倒序）。
type PresentIndexRequest struct {
	protocol.RequestBase
	TimeFilter int  `msgpack:"time_filter" json:"time_filter"`
	TypeFilter int  `msgpack:"type_filter" json:"type_filter"`
	DescFlag   bool `msgpack:"desc_flag" json:"desc_flag"`
	Offset     int  `msgpack:"offset" json:"offset"`
}

func (*PresentIndexRequest) URL() *url.URL { return urlPresentIndex }

// PresentParameter 是礼物箱中的一件礼物（此处仅取所需字段）。
type PresentParameter struct {
	PresentID   int `msgpack:"present_id" json:"present_id"`
	RewardType  int `msgpack:"reward_type" json:"reward_type"`
	RewardID    int `msgpack:"reward_id" json:"reward_id"`
	RewardCount int `msgpack:"reward_count" json:"reward_count"`
}

// PresentIndexResponse 携带礼物箱内容列表（其余字段由解码器忽略）。
type PresentIndexResponse struct {
	protocol.ResponseBase
	PresentInfoList []PresentParameter `msgpack:"present_info_list" json:"present_info_list"`
	PresentCount    int                `msgpack:"present_count" json:"present_count"`
}

// PresentReceiveAllRequest 领取礼物箱中符合过滤条件的全部礼物。
//
// time_filter=-1 / type_filter=0 / desc_flag=true 复刻权威客户端的"全部、不限时、倒序"；
// is_exclude_stamina=true 时跳过体力饮料礼物（避免体力溢出浪费）。
type PresentReceiveAllRequest struct {
	protocol.RequestBase
	TimeFilter       int  `msgpack:"time_filter" json:"time_filter"`
	TypeFilter       int  `msgpack:"type_filter" json:"type_filter"`
	DescFlag         bool `msgpack:"desc_flag" json:"desc_flag"`
	IsExcludeStamina bool `msgpack:"is_exclude_stamina" json:"is_exclude_stamina"`
}

func (*PresentReceiveAllRequest) URL() *url.URL { return urlPresentReceiveAll }

// PresentReceiveAllResponse 携带本次领取到的奖励列表（其余字段由解码器忽略）。
type PresentReceiveAllResponse struct {
	StaminaInfo *protocol.UserStaminaInfo `msgpack:"stamina_info" json:"stamina_info"`
	protocol.ResponseBase
	Rewards []protocol.InventoryInfo `msgpack:"rewards" json:"rewards"`
}

// InventoryChanges 实现 protocol.RewardCarrier——领取回传的奖励条目带 stock（真机实测）。
func (r *PresentReceiveAllResponse) InventoryChanges() []protocol.InventoryInfo { return r.Rewards }

// StaminaSnapshot 实现 protocol.StaminaCarrier——领取体力药会当场改变体力。
func (r *PresentReceiveAllResponse) StaminaSnapshot() *protocol.UserStaminaInfo {
	return r.StaminaInfo
}
