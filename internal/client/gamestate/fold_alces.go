package gamestate

import (
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
)

// 彩装究极炼成域的折叠器：exec 出 pending 副属性、fix_result 定案、lock_slot 锁定。
// subStatusFromAlces 把 alces 协议副属性转为 gamestate 形态。
func subStatusFromAlces(ss []alces.SubStatus) []ExEquipSubStatus {
	out := make([]ExEquipSubStatus, len(ss))
	for i, s := range ss {
		out[i] = ExEquipSubStatus{SlotNumber: s.SlotNumber, Status: s.Status, Step: s.Step, IsLock: s.IsLock}
	}
	return out
}

// foldAlcesExec 把 exec 回传的炼成 PT 余量与金币余额折进状态（pending 副属性尚未定案，不改装备）。
// 金币必须折回：模块每发 exec 都要带 current_gold 快照，漏折则第二发起就是过期值。
func foldAlcesExec(s *PlayerState, _ protocol.Request, resp any) {
	r := resp.(*alces.ExecResponse)
	if r.CurrentAlcesPoint != nil && s.Inventory != nil {
		p := r.CurrentAlcesPoint
		s.Inventory[InventoryKey{Type: p.Type, ID: p.ID}] = p.Stock
	}
	if r.UserGold != nil {
		s.Gold = Currency{Free: r.UserGold.GoldIDFree, Paid: r.UserGold.GoldIDPay}
	}
}

// foldAlcesFixResult 用定案回传的完整实例更新对应彩装（副属性/锁定态随之刷新）。
func foldAlcesFixResult(s *PlayerState, _ protocol.Request, resp any) {
	r := resp.(*alces.FixResultResponse)
	if r.FixedAlcesData == nil || s.ExEquips == nil {
		return
	}
	e := r.FixedAlcesData
	s.ExEquips[e.SerialID] = ExEquip{
		SerialID:       e.SerialID,
		ExEquipmentID:  e.ExEquipmentID,
		EnhancementPt:  e.EnhancementPt,
		Rank:           e.Rank,
		ProtectionFlag: e.ProtectionFlag,
		SubStatus:      subStatusFromAlces(e.SubStatus),
		IsAlcesPending: e.IsAlcesPending != 0,
	}
}

// foldAlcesLockSlot 用锁定后的炼成数据列表更新对应彩装的副属性（锁定标志）。
func foldAlcesLockSlot(s *PlayerState, _ protocol.Request, resp any) {
	r := resp.(*alces.LockSlotResponse)
	if s.ExEquips == nil {
		return
	}
	for _, d := range r.AlcesDataList {
		e, ok := s.ExEquips[d.SerialID]
		if !ok {
			continue
		}
		e.SubStatus = subStatusFromAlces(d.SubStatus)
		s.ExEquips[d.SerialID] = e
	}
}
