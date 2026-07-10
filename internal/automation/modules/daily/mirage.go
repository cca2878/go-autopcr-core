package daily

import (
	"context"
	"fmt"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

const (
	mirageUnlockQuestID = 11072001 // 追忆战解锁任务
	mirageSystemID      = 135      // 追忆战来源系统 id（eSystemId.MIRAGE）
	secondsPerDay       = 24 * 60 * 60
)

// mirageFloorReceive 领取追忆战礼物池奖励（对应 ref mirage_floor_receive，先查后领）。
type mirageFloorReceive struct{}

func (mirageFloorReceive) Meta() automation.Meta {
	return automation.Meta{
		Name:            "mirage_floor_receive",
		Title:           "追忆战礼物收取",
		Description:     "领取追忆战礼物池已累积的奖励（不消耗资源）",
		Category:        "收取",
		NeedsMasterdata: true,
	}
}

func (mirageFloorReceive) Params() []automation.Param { return nil }

func (mirageFloorReceive) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("追忆战礼物收取需要母数据，但未启用")
	}
	// 先查后动：未解锁追忆战时 mirage/top 会触发业务错误，据登录折叠的任务状态先行拦截。
	if !gc.Data().IsQuestCleared(mirageUnlockQuestID) {
		return automation.Skip("追忆战未解锁")
	}
	fullTime, err := gc.Mirage().RewardFullTime(ctx)
	if err != nil {
		return err
	}
	if fullTime == -1 {
		return automation.Skip("追忆战未通关")
	}
	days, err := md.Mirage().AccumulateDayMax(ctx)
	if err != nil {
		return err
	}
	// 复刻 ref：距礼物池注满不足 (上限天数-1) 天，说明池中已有可领奖励。
	now := gc.ServerTime()
	if fullTime-now >= int64(days-1)*secondsPerDay {
		return automation.Skip("礼物箱无奖励")
	}
	count, err := gc.Mirage().ReceiveReward(ctx, mirageSystemID)
	if err != nil {
		return err
	}
	if count == 0 {
		return automation.Skip("礼物箱无奖励")
	}
	rc.Logf("领取追忆战礼物 %d 件", count)
	return nil
}
