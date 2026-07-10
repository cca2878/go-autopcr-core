package sweep

import (
	"context"
	"fmt"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// dungeonReport 报告地下城今日是否还可挑战/扫荡（只读，不实际进入/扫荡）。
//
// 对应 ref underground_skip（地下城扫荡）：原模块会进入并扫荡地下城换取 mana；此处降级为只读
// 报告，只据 dungeon/info 的剩余挑战次数与当前所在区域给出状态，不发送进入/扫荡写请求。
type dungeonReport struct{}

func (dungeonReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "dungeon",
		Title:           "地下城（可扫荡报告）",
		Description:     "报告地下城今日剩余挑战次数与当前所在区域（只读，不实际进入/扫荡）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (dungeonReport) Params() []automation.Param { return nil }

func (dungeonReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("地下城报告需要母数据解析区域名，但未启用")
	}
	info, err := gc.Dungeon().Info(ctx)
	if err != nil {
		return err
	}
	// 当前所在区域（若在地下城内）。
	if info.EnterAreaID > 0 {
		name, err := md.Dungeon().AreaName(ctx, info.EnterAreaID)
		if err != nil {
			return err
		}
		if name == "" {
			name = fmt.Sprintf("区域 %d", info.EnterAreaID)
		}
		rc.Logf("当前位于地下城：%s", name)
	}
	if info.RestChallenge <= 0 {
		return automation.Skip("今日地下城已无可挑战次数")
	}
	rc.Logf("地下城今日剩余可挑战：%d 次（上限 %d）", info.RestChallenge, info.MaxChallenge)
	return nil
}
