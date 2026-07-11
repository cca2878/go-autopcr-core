package daily

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// 竞技场/公主竞技场的解锁任务 id（对应 ref get_arena_info / get_grand_arena_info 的门禁）。
const (
	arenaUnlockQuestID      = 11004006
	grandArenaUnlockQuestID = 11008015
)

// jjcReward 领取竞技场与公主竞技场的时间奖励（jjc/pjjc 币，不消耗资源）。
type jjcReward struct{}

func (jjcReward) Meta() automation.Meta {
	return automation.Meta{
		Name:        "jjc_reward",
		Title:       "领取双场币",
		Description: "领取竞技场与公主竞技场的时间奖励币（不消耗资源）",
		Category:    "收取",
	}
}

func (jjcReward) Params() []automation.Param { return nil }

func (jjcReward) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	data := gc.Data()
	got := false

	// 竞技场：先查解锁再查可领数量，仅在确有可领时才领取（避免触发业务错误）。
	if data.IsQuestCleared(arenaUnlockQuestID) {
		count, err := gc.Arena().ArenaRewardCount(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			if err := gc.Arena().ReceiveArenaReward(ctx); err != nil {
				return err
			}
			rc.Logf("领取竞技场币 x%d", count)
			got = true
		}
	}

	// 公主竞技场：同上。
	if data.IsQuestCleared(grandArenaUnlockQuestID) {
		count, err := gc.Arena().GrandArenaRewardCount(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			if err := gc.Arena().ReceiveGrandArenaReward(ctx); err != nil {
				return err
			}
			rc.Logf("领取公主竞技场币 x%d", count)
			got = true
		}
	}

	if !got {
		return automation.Skip("无可领取的双场币")
	}
	return nil
}
