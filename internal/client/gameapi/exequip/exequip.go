// Package exequip 是"EX 装备（含彩装究极炼成）"域的游戏 API 能力面（对应参考项目 alces_* 调用）。
//
// 炼成是一次性重掷未锁副属性的 gacha：AlcesExec 产出待决定数据，AlcesFixResult 采纳、
// AlcesCancelResult 放弃、AlcesLockSlot 改锁定。是否解锁（究极炼成 quest）由上层据玩家任务状态
// 判定；本域只负责发包。装备/PT 的本地态由客户端折叠中间件在响应后自动更新。
package exequip

import (
	"context"

	alcespb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// SubStatus 是一条炼成副属性（gameapi 结果类型）。status＝属性类型(eParamType)，step＝档位(1..5)。
type SubStatus struct {
	SlotNumber int
	Status     int
	Step       int
	IsLock     bool
}

// AlcesPending 是一份待决定的炼成数据（serial_id + 本次掷出的副属性）。
type AlcesPending struct {
	SerialID  int
	SubStatus []SubStatus
}

// API 是 EX 装备域能力面契约（随功能在本包内累加）。
type API interface {
	// AlcesTop 拉取炼成首页；返回待决定数据（无待决定＝nil）。
	AlcesTop(ctx context.Context) (*AlcesPending, error)
	// AlcesExec 对某彩装执行一次究极炼成；currentPt/currentGold 为客户端侧 PT/mana 快照（服务端校验用）。
	// 返回本次掷出的待决定数据。
	AlcesExec(ctx context.Context, serialID, currentPt, currentGold int) (*AlcesPending, error)
	// AlcesFixResult 采纳上次炼成结果（定案；装备副属性随定案实例经折叠更新）。
	AlcesFixResult(ctx context.Context, serialID int) error
	// AlcesCancelResult 放弃上次炼成结果（回退到重掷前）。
	AlcesCancelResult(ctx context.Context, serialID int) error
	// AlcesLockSlot 设置某彩装某槽的锁定态（true＝锁）。
	AlcesLockSlot(ctx context.Context, serialID, slotNumber int, lock bool) error
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造 EX 装备域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

// pendingFrom 把协议层 AlcesData 转为域结果类型（nil→nil）。
func pendingFrom(d *alcespb.AlcesData) *AlcesPending {
	if d == nil {
		return nil
	}
	subs := make([]SubStatus, len(d.SubStatus))
	for i, s := range d.SubStatus {
		subs[i] = SubStatus{SlotNumber: s.SlotNumber, Status: s.Status, Step: s.Step, IsLock: s.IsLock}
	}
	return &AlcesPending{SerialID: d.SerialID, SubStatus: subs}
}

func (a *Impl) AlcesTop(ctx context.Context) (*AlcesPending, error) {
	resp, err := transport.Call[alcespb.TopResponse](ctx, a.tr, &alcespb.TopRequest{})
	if err != nil {
		return nil, err
	}
	return pendingFrom(resp.PendingAlcesData), nil
}

func (a *Impl) AlcesExec(ctx context.Context, serialID, currentPt, currentGold int) (*AlcesPending, error) {
	resp, err := transport.Call[alcespb.ExecResponse](ctx, a.tr, &alcespb.ExecRequest{
		SerialID:          serialID,
		CurrentAlcesPoint: currentPt,
		CurrentGold:       currentGold,
	})
	if err != nil {
		return nil, err
	}
	return pendingFrom(resp.PendingAlcesData), nil
}

func (a *Impl) AlcesFixResult(ctx context.Context, serialID int) error {
	_, err := transport.Call[alcespb.FixResultResponse](ctx, a.tr, &alcespb.FixResultRequest{SerialID: serialID})
	return err
}

func (a *Impl) AlcesCancelResult(ctx context.Context, serialID int) error {
	_, err := transport.Call[alcespb.CancelResultResponse](ctx, a.tr, &alcespb.CancelResultRequest{SerialID: serialID})
	return err
}

func (a *Impl) AlcesLockSlot(ctx context.Context, serialID, slotNumber int, lock bool) error {
	isLock := 0
	if lock {
		isLock = 1
	}
	_, err := transport.Call[alcespb.LockSlotResponse](ctx, a.tr, &alcespb.LockSlotRequest{
		LockList: []alcespb.AlcesDataPost{{
			SerialID:  serialID,
			SubStatus: []alcespb.SubStatusPost{{SlotNumber: slotNumber, IsLock: isLock}},
		}},
	})
	return err
}
