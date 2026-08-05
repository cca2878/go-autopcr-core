// Package alces 是"彩装究极炼成"域的 DTO（alces/top、exec、fix_result、cancel_result、
// lock_slot；对应参考项目 AlcesXxxRequest/Response）。炼成是一次性重掷未锁副属性的 gacha：exec 产出
// 待决定的 pending_alces_data，fix_result 定案回传完整实例，cancel_result 放弃回退，lock_slot 改锁定。
package alces

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlTop          = protocol.MustRelURL("alces/top")
	urlExec         = protocol.MustRelURL("alces/exec")
	urlFixResult    = protocol.MustRelURL("alces/fix_result")
	urlCancelResult = protocol.MustRelURL("alces/cancel_result")
	urlLockSlot     = protocol.MustRelURL("alces/lock_slot")
)

// SubStatus 是一条 EX 装备副属性（对应参考项目 ExtraEquipSubStatus）。status＝属性类型(eParamType)，
// step＝档位(1..5，5＝满)，is_lock＝是否锁定。
type SubStatus struct {
	SlotNumber int  `msgpack:"slot_number" json:"slot_number"`
	Status     int  `msgpack:"status" json:"status"`
	Step       int  `msgpack:"step" json:"step"`
	IsLock     bool `msgpack:"is_lock" json:"is_lock"`
}

// AlcesData 是一件待决定/锁定的炼成数据（serial_id + 副属性列表）。
type AlcesData struct {
	SerialID  int         `msgpack:"serial_id" json:"serial_id"`
	SubStatus []SubStatus `msgpack:"sub_status" json:"sub_status"`
}

// ExtraEquipInfo 是炼成定案(fix_result)后回传的完整 EX 装备实例（对应参考项目 ExtraEquipInfo）。
type ExtraEquipInfo struct {
	SerialID       int         `msgpack:"serial_id" json:"serial_id"`
	ExEquipmentID  int         `msgpack:"ex_equipment_id" json:"ex_equipment_id"`
	EnhancementPt  int         `msgpack:"enhancement_pt" json:"enhancement_pt"`
	Rank           int         `msgpack:"rank" json:"rank"`
	ProtectionFlag int         `msgpack:"protection_flag" json:"protection_flag"`
	SubStatus      []SubStatus `msgpack:"sub_status" json:"sub_status"`
	IsAlcesPending int         `msgpack:"is_alces_pending" json:"is_alces_pending"`
}

// SubStatusPost 是锁定请求的一条上行副属性（对应参考项目 ExtraEquipSubStatusPost；is_lock 用 int）。
type SubStatusPost struct {
	SlotNumber int `msgpack:"slot_number" json:"slot_number"`
	IsLock     int `msgpack:"is_lock" json:"is_lock"`
}

// AlcesDataPost 是锁定请求的上行体（对应参考项目 AlcesDataPost）。
type AlcesDataPost struct {
	SerialID  int             `msgpack:"serial_id" json:"serial_id"`
	SubStatus []SubStatusPost `msgpack:"sub_status" json:"sub_status"`
}

// TopRequest 拉取炼成首页（含可能存在的待决定数据）。
type TopRequest struct{ protocol.RequestBase }

func (*TopRequest) URL() *url.URL { return urlTop }

// TopResponse 携带待决定的炼成数据（pending_alces_data 非 nil＝上次 exec 尚未决定）。
type TopResponse struct {
	protocol.ResponseBase
	PendingAlcesData *AlcesData `msgpack:"pending_alces_data" json:"pending_alces_data"`
}

// ExecRequest 对某彩装执行一次究极炼成（重掷未锁副属性）。current_alces_point/current_gold 为
// 客户端侧的当前 PT/mana 快照（服务端校验用）。
type ExecRequest struct {
	protocol.RequestBase
	SerialID          int `msgpack:"serial_id" json:"serial_id"`
	CurrentAlcesPoint int `msgpack:"current_alces_point" json:"current_alces_point"`
	CurrentGold       int `msgpack:"current_gold" json:"current_gold"`
}

func (*ExecRequest) URL() *url.URL { return urlExec }

// ExecResponse 携带本次掷出的待决定副属性(pending_alces_data)、炼成 PT 余量(current_alces_point)
// 与扣费后的金币余额(user_gold)——后者须折回状态，否则下一发 exec 会带着过期的 current_gold 快照。
type ExecResponse struct {
	protocol.ResponseBase
	PendingAlcesData  *AlcesData              `msgpack:"pending_alces_data" json:"pending_alces_data"`
	CurrentAlcesPoint *protocol.InventoryInfo `msgpack:"current_alces_point" json:"current_alces_point"`
	UserGold          *protocol.UserGold      `msgpack:"user_gold" json:"user_gold"`
}

// FixResultRequest 采纳上次 exec 的结果（定案）。
type FixResultRequest struct {
	protocol.RequestBase
	SerialID int `msgpack:"serial_id" json:"serial_id"`
}

func (*FixResultRequest) URL() *url.URL { return urlFixResult }

// FixResultResponse 回传定案后的完整装备实例(fixed_alces_data)——据此更新本地装备副属性。
type FixResultResponse struct {
	protocol.ResponseBase
	FixedAlcesData *ExtraEquipInfo `msgpack:"fixed_alces_data" json:"fixed_alces_data"`
}

// CancelResultRequest 放弃上次 exec 的结果（回退到重掷前）。
type CancelResultRequest struct {
	protocol.RequestBase
	SerialID int `msgpack:"serial_id" json:"serial_id"`
}

func (*CancelResultRequest) URL() *url.URL { return urlCancelResult }

// CancelResultResponse 仅回带 serial_id（装备副属性未变）。
type CancelResultResponse struct {
	protocol.ResponseBase
	SerialID int `msgpack:"serial_id" json:"serial_id"`
}

// LockSlotRequest 批量设置某彩装若干槽的锁定态（lock_list 每项一件装的若干槽）。
type LockSlotRequest struct {
	protocol.RequestBase
	LockList []AlcesDataPost `msgpack:"lock_list" json:"lock_list"`
}

func (*LockSlotRequest) URL() *url.URL { return urlLockSlot }

// LockSlotResponse 回带更新锁定态后的炼成数据列表(alces_data_list)——据此更新本地锁定标志。
type LockSlotResponse struct {
	protocol.ResponseBase
	AlcesDataList []AlcesData `msgpack:"alces_data_list" json:"alces_data_list"`
}
