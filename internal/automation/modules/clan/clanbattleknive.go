package clan

import (
	"context"
	"fmt"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// clanBattlePointFull 是体力点数满值（三刀出完时为满且无剩余刀/尾刀）。
const clanBattlePointFull = 900

// clanBattleKnive 报告公会战今日剩余刀数/尾刀/体力点数（只读；对应参考项目 clan_battle_knive）。
type clanBattleKnive struct{}

func (clanBattleKnive) Meta() automation.Meta {
	return automation.Meta{
		Name:        "clan_battle_knive",
		Title:       "公会战刀数",
		Description: "报告公会战今日剩余整刀/尾刀与体力点数（只读）",
		Category:    "公会",
	}
}

func (clanBattleKnive) Params() []automation.Param { return nil }

func (clanBattleKnive) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	data := gc.Data()
	// 先查后动：未加入公会时 clan_battle/top 会触发业务错误，故据登录折叠的公会状态先行拦截。
	if data.ClanID == 0 {
		return automation.Skip("未加入公会")
	}
	top, err := gc.ClanBattle().Top(ctx, data.ClanID)
	if err != nil {
		return err
	}
	if top.RemainingCount == 0 && top.Point == clanBattlePointFull && len(top.CarryOverTimes) == 0 {
		return automation.Skip("今日三刀已出完")
	}

	var parts []string
	if top.RemainingCount > 0 {
		parts = append(parts, fmt.Sprintf("剩余整刀：%d", top.RemainingCount))
	}
	if len(top.CarryOverTimes) > 0 {
		times := make([]string, len(top.CarryOverTimes))
		for i, t := range top.CarryOverTimes {
			times[i] = fmt.Sprintf("%d秒", t)
		}
		parts = append(parts, fmt.Sprintf("剩余尾刀：%d（%s）", len(top.CarryOverTimes), strings.Join(times, "、")))
	}
	parts = append(parts, fmt.Sprintf("体力点数：%d", top.Point))
	rc.Logf("%s", strings.Join(parts, " | "))
	return nil
}
