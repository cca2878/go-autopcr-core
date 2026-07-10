// Package room 汇集「家园」域的自动化模块（收取家园产物）。
package room

import (
	"context"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(roomAccept{})
	r.Register(loveUpReport{})
}

// roomAccept 收取家园全部待收产物（mana/体力等，不消耗资源）。
type roomAccept struct{}

func (roomAccept) Meta() automation.Meta {
	return automation.Meta{
		Name:        "room",
		Title:       "收取家园产物",
		Description: "一键收取家园全部待收产物（mana/体力等，不消耗资源）",
		Category:    "收取",
	}
}

func (roomAccept) Params() []automation.Param { return nil }

func (roomAccept) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	// 先查后收：进入家园读取家具状态，仅在确有待收产物时才 receive_all，避免触发业务错误。
	items, err := gc.Room().Start(ctx)
	if err != nil {
		return err
	}
	has := false
	for _, it := range items {
		if it.ItemCount > 0 {
			has = true
			break
		}
	}
	if !has {
		return automation.Skip("没有可收取的家园产物")
	}
	rewards, err := gc.Room().ReceiveAll(ctx)
	if err != nil {
		return err
	}
	rc.Logf("收取了 %d 件家园产物", len(rewards))
	return nil
}
