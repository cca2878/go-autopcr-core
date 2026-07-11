// Package sweep 汇集「扫荡」域的自动化模块（探索/地下城等扫荡；对应 ref sweep.py）。
package sweep

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(exploreExpReport{})
	r.Register(exploreManaReport{})
	r.Register(dungeonReport{})
	r.Register(towerCloisterReport{})
}

// exploreExpReport 报告今日 EXP 探索还可扫荡多少次（只读，不实际扫荡/消耗体力）。
//
// 对应 ref explore_exp：原模块会扫荡消耗体力换取经验药水；此处降级为只读报告，只据登录折叠
// 的今日已用/上限次数报告剩余可扫荡次数，不发送 training_quest/skip 写请求。
type exploreExpReport struct{}

func (exploreExpReport) Meta() automation.Meta {
	return automation.Meta{
		Name:        "explore_exp",
		Title:       "EXP探索（可扫荡报告）",
		Description: "报告今日 EXP 探索剩余可扫荡次数（只读，不实际扫荡/消耗体力）",
		Category:    "查询",
	}
}

func (exploreExpReport) Params() []automation.Param { return nil }

func (exploreExpReport) Run(_ context.Context, gc client.GameClient, rc *automation.RunContext) error {
	data := gc.Data()
	remain := data.TrainingExpMax - data.TrainingExpDone
	if remain <= 0 {
		return automation.Skip("今日 EXP 探索已扫荡完")
	}
	rc.Logf("EXP 探索今日剩余可扫荡：%d 次（已用 %d / 上限 %d）", remain, data.TrainingExpDone, data.TrainingExpMax)
	return nil
}
