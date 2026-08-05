package daily

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	gapidaily "github.com/cca2878/go-autopcr-core/internal/client/gameapi/daily"
)

// maxReceiveBatches 是"按批领取"类任务的循环上限，防止意外死循环（实际一般 1~2 批即清空）。
const maxReceiveBatches = 20

// presentReceive 领取礼物箱中的奖励（只领已获得、不消耗任何资源）。
type presentReceive struct{}

func (presentReceive) Meta() automation.Meta {
	return automation.Meta{
		Name:        "present",
		Title:       "领取礼物箱",
		Description: "领取礼物箱中的奖励（不消耗资源）",
		Category:    "收取",
	}
}

func (presentReceive) Params() []automation.Param {
	return []automation.Param{
		{Name: "exclude_stamina", Type: automation.ParamBool, Default: true, Description: "跳过体力饮料礼物，避免体力溢出浪费"},
	}
}

func (presentReceive) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	exclude := rc.Bool("exclude_stamina")
	var total []gapidaily.Reward
	// 先查后领：只在礼物箱确有可领取礼物时才 receive，避免触发"已领取"等业务错误。
	for range maxReceiveBatches {
		box, err := gc.Daily().PresentBox(ctx)
		if err != nil {
			return err
		}
		if !hasReceivable(box, exclude) {
			break
		}
		rewards, err := gc.Daily().ReceiveAllPresents(ctx, exclude)
		if err != nil {
			return err
		}
		if len(rewards) == 0 {
			break // 领不动了（如背包已满），停止避免死循环
		}
		total = append(total, rewards...)
	}
	if len(total) == 0 {
		return automation.Skip("没有可领取的礼物")
	}
	rc.Logf("领取了 %d 件奖励", len(total))
	return nil
}

// hasReceivable 判断礼物箱里是否有本次会去领取的礼物（exclude 时排除体力饮料）。
func hasReceivable(box []gapidaily.Present, exclude bool) bool {
	for _, p := range box {
		if !exclude || !p.IsStaminaDrink() {
			return true
		}
	}
	return false
}
