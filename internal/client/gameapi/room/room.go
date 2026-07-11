// Package room 是「家园」域的游戏 API 能力面（家园产物收取等；对应 ref room.py）。
//
// 能力方法按具体系统分文件，共用同一 Impl（只持传输句柄）。
package room

import (
	"context"

	roompb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/room"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// API 是家园域能力面契约（随功能在本包内累加）。
type API interface {
	// Start 进入家园并返回家具列表（含各家具的待收产物计数）。供先查后收。
	Start(ctx context.Context) ([]Item, error)
	// ReceiveAll 一键收取家园全部待收产物，返回收取到的产物列表。
	ReceiveAll(ctx context.Context) ([]Reward, error)
}

// Item 是家园中一件家具及其当前待收产物数量。
type Item struct {
	SerialID   int
	RoomItemID int
	ItemCount  int // >0 表示有待收产物
}

// Reward 是一次收取到的产物：库存类型 + 物品 id + 数量。名称解析（依赖母数据）待后续增强。
type Reward struct {
	Type  int
	ID    int
	Count int
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造家园域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Start(ctx context.Context) ([]Item, error) {
	resp, err := transport.Call[roompb.RoomStartResponse](ctx, a.tr, &roompb.RoomStartRequest{WacAutoOptionFlag: 1})
	if err != nil {
		return nil, err
	}
	out := make([]Item, len(resp.UserRoomItemList))
	for i, it := range resp.UserRoomItemList {
		out[i] = Item{SerialID: it.SerialID, RoomItemID: it.RoomItemID, ItemCount: it.ItemCount}
	}
	return out, nil
}

func (a *Impl) ReceiveAll(ctx context.Context) ([]Reward, error) {
	resp, err := transport.Call[roompb.RoomReceiveItemAllResponse](ctx, a.tr, &roompb.RoomReceiveItemAllRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]Reward, len(resp.RewardList))
	for i, r := range resp.RewardList {
		out[i] = Reward{Type: r.Type, ID: r.ID, Count: r.Count}
	}
	return out, nil
}
