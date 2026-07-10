package daily

import (
	"context"
	"fmt"
	"time"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// seasonpassAccept 领取进行中女神祭的任务奖励（对应 ref seasonpass_accept）。
type seasonpassAccept struct{}

func (seasonpassAccept) Meta() automation.Meta {
	return automation.Meta{
		Name:            "seasonpass_accept",
		Title:           "领取女神祭任务",
		Description:     "领取进行中女神祭的任务奖励（不消耗资源）",
		Category:        "收取",
		NeedsMasterdata: true,
	}
}

func (seasonpassAccept) Params() []automation.Param { return nil }

func (seasonpassAccept) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("女神祭需要母数据判定开放期，但未启用")
	}
	seasons, err := md.Seasonpass().ActiveSeasonIDs(ctx, time.Unix(gc.ServerTime(), 0))
	if err != nil {
		return err
	}
	if len(seasons) == 0 {
		return automation.Skip("目前无进行中的女神祭")
	}

	got := false
	for _, seasonID := range seasons {
		// 先查后领：仅在确有可领任务时才 accept，避免触发业务错误。
		idx, err := gc.Seasonpass().Index(ctx, seasonID)
		if err != nil {
			return err
		}
		if !idx.HasReceivableTask {
			continue
		}
		count, level, err := gc.Seasonpass().AcceptMissions(ctx, seasonID)
		if err != nil {
			return err
		}
		rc.Logf("领取女神祭任务奖励 %d 件，当前女神祭等级：%d", count, level)
		got = true
	}
	if !got {
		return automation.Skip("没有可领取的女神祭任务")
	}
	return nil
}
