// Package account 汇集「账号/首页」域的自动化模块（概览、刷新首页）。
package account

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(summary{})
	r.Register(home{})
}

// summary 只读打印已聚合的玩家档案（零请求、零账号改动）。
type summary struct{}

func (summary) Meta() automation.Meta {
	return automation.Meta{
		Name:        "summary",
		Title:       "账号概览",
		Description: "读取登录时已聚合的玩家档案（不发任何请求）",
		Category:    "只读",
	}
}

func (summary) Params() []automation.Param {
	return []automation.Param{
		{Name: "compact", Type: automation.ParamBool, Default: false, Description: "紧凑输出（合并为一行）"},
	}
}

func (summary) Run(_ context.Context, gc client.GameClient, rc *automation.RunContext) error {
	d := gc.Data()
	if rc.Bool("compact") {
		rc.Logf("%s Lv%d ｜ 体力%d 金币%d 钻石%d", d.UserName, d.TeamLevel, d.Stamina, d.Gold, d.Jewel.Total)
		return nil
	}
	rc.Logf("昵称 %s（等级 %d）", d.UserName, d.TeamLevel)
	rc.Logf("体力 %d ｜ 金币 %d ｜ 钻石 %d（免费 %d）", d.Stamina, d.Gold, d.Jewel.Total, d.Jewel.Free)
	return nil
}

// home 重新拉取首页（幂等读），验证「账号域能力面被模块驱动」这条链路。
type home struct{}

func (home) Meta() automation.Meta {
	return automation.Meta{
		Name:        "home",
		Title:       "刷新首页",
		Description: "重新拉取 load/index 刷新玩家状态（幂等读）",
		Category:    "只读",
	}
}

func (home) Params() []automation.Param { return nil }

func (home) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	if err := gc.Account().RefreshIndex(ctx); err != nil {
		return err
	}
	d := gc.Data()
	rc.Logf("首页已刷新：体力 %d ｜ 金币 %d", d.Stamina, d.Gold)
	return nil
}
