package daily

import (
	"context"
	"slices"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	gapidaily "github.com/cca2878/go-autopcr-core/internal/client/gameapi/daily"
)

// missionReceive 领取任务奖励（日常/常驻/纹章，不消耗资源）。
//
// 领取接口 mission/accept 只接受'类别'(type=1/2/4)而非单个任务 id，故需母数据把待领任务
// 归类，只对确有可领任务的类别发起领取（先查+分类+领），避免对空类别触发业务错误。
type missionReceive struct{}

func (missionReceive) Meta() automation.Meta {
	return automation.Meta{
		Name:            "mission",
		Title:           "领取任务奖励",
		Description:     "领取日常/常驻/纹章任务奖励（不消耗资源）",
		Category:        "收取",
		NeedsMasterdata: true, // 需据母数据把待领任务归类到 type 1/2/4
	}
}

func (missionReceive) Params() []automation.Param { return nil }

func (missionReceive) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return automation.RequireMasterdata("给待领任务分类")
	}
	cls, err := md.Mission().Classifier(ctx)
	if err != nil {
		return err
	}
	missions, err := gc.Daily().MissionList(ctx)
	if err != nil {
		return err
	}
	// 先查后领：只收集"确有可领取任务"的类别，再逐类别领取。
	var categories []int
	for _, m := range missions {
		if !m.Receivable {
			continue
		}
		if cat := cls.Type(m.ID); cat != 0 && !slices.Contains(categories, cat) {
			categories = append(categories, cat)
		}
	}
	if len(categories) == 0 {
		return automation.Skip("没有可领取的任务奖励")
	}
	slices.Sort(categories)
	var total []gapidaily.Reward
	for _, cat := range categories {
		rewards, err := gc.Daily().AcceptMissions(ctx, cat)
		if err != nil {
			return err
		}
		total = append(total, rewards...)
	}
	rc.Logf("领取了 %d 项任务奖励", len(total))
	return nil
}
