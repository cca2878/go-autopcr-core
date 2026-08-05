package sweep

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// towerUnlockQuestID 是露娜塔解锁任务（对应参考项目 get_tower_top 的门禁）。
const towerUnlockQuestID = 11009001

// towerCloisterReport 报告露娜塔回廊今日是否还可扫荡（只读；对应参考项目 tower_cloister_sweep 降级为报告）。
//
// 原模块会扫荡回廊换取奖励；此处降级为只读报告，只据开放窗口 + 回廊状态给出剩余可扫荡次数，
// 不发送 cloister_battle/skip 写请求。
type towerCloisterReport struct{}

func (towerCloisterReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "tower_cloister",
		Title:           "露娜塔回廊（可扫荡报告）",
		Description:     "报告露娜塔回廊今日剩余可扫荡次数（只读，不实际扫荡）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (towerCloisterReport) Params() []automation.Param { return nil }

func (towerCloisterReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return automation.RequireMasterdata("判定露娜塔开放期")
	}
	// 先查后动：未解锁露娜塔时 tower/top 会触发业务错误，据登录折叠的任务状态先行拦截。
	if !gc.Data().IsQuestCleared(towerUnlockQuestID) {
		return automation.Skip("未解锁露娜塔")
	}
	start, end, ok, err := md.Tower().NewestWindow(ctx)
	if err != nil {
		return err
	}
	now := time.Unix(gc.ServerTime(), 0)
	if !ok || now.Before(start) {
		return automation.Skip("露娜塔未开启")
	}
	if now.After(end) {
		return automation.Skip("露娜塔已结束")
	}
	top, err := gc.Tower().Top(ctx)
	if err != nil {
		return err
	}
	if !top.CloisterFirstCleared {
		return automation.Skip("回廊首关未通关")
	}
	if top.CloisterRemainClear <= 0 {
		return automation.Skip("回廊今日已扫荡")
	}
	rc.Logf("露娜塔回廊今日剩余可扫荡：%d 次", top.CloisterRemainClear)
	return nil
}
