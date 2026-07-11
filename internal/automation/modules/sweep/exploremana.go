package sweep

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// exploreManaReport 报告今日 Mana 探索还可扫荡多少次（只读，不实际扫荡/消耗体力）。
//
// 对应 ref explore_mana：原模块扫荡消耗体力换取 mana；此处与 exploreExpReport 同法降级为只读
// 报告，只据登录折叠的今日已用/上限次数报告剩余可扫荡次数。
type exploreManaReport struct{}

func (exploreManaReport) Meta() automation.Meta {
	return automation.Meta{
		Name:        "explore_mana",
		Title:       "Mana探索（可扫荡报告）",
		Description: "报告今日 Mana 探索剩余可扫荡次数（只读，不实际扫荡/消耗体力）",
		Category:    "查询",
	}
}

func (exploreManaReport) Params() []automation.Param { return nil }

func (exploreManaReport) Run(_ context.Context, gc client.GameClient, rc *automation.RunContext) error {
	data := gc.Data()
	remain := data.TrainingManaMax - data.TrainingManaDone
	if remain <= 0 {
		return automation.Skip("今日 Mana 探索已扫荡完")
	}
	rc.Logf("Mana 探索今日剩余可扫荡：%d 次（已用 %d / 上限 %d）", remain, data.TrainingManaDone, data.TrainingManaMax)
	return nil
}
