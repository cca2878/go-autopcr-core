// Package clan 汇集「公会」域的自动化模块（公会点赞…）。
package clan

import (
	"context"
	"math/rand"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	gapiclan "github.com/cca2878/go-autopcr-core/internal/client/gameapi/clan"
)

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(clanLike{})
	r.Register(clanBattleKnive{})
}

// clanLike 在公会中随机挑一名成员点赞（每日一次，不消耗资源）。
type clanLike struct{}

func (clanLike) Meta() automation.Meta {
	return automation.Meta{
		Name:        "clan_like",
		Title:       "公会点赞",
		Description: "在公会中随机选择一位成员点赞（每日一次，不消耗资源）",
		Category:    "公会",
	}
}

func (clanLike) Params() []automation.Param { return nil }

func (clanLike) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	data := gc.Data()
	// 先查后动：从登录折叠的状态判断前置条件，满足才发点赞请求，避免触发业务错误。
	if data.ClanID == 0 {
		return automation.Skip("未加入公会")
	}
	if data.ClanLikeCount > 0 {
		return automation.Skip("今日已点赞")
	}
	members, err := gc.Clan().Members(ctx, data.ClanID)
	if err != nil {
		return err
	}
	// 排除自己，只在其他成员中挑选。
	others := make([]gapiclan.Member, 0, len(members))
	for _, m := range members {
		if m.ViewerID != data.ViewerID {
			others = append(others, m)
		}
	}
	if len(others) == 0 {
		return automation.Skip("公会内没有其他成员可点赞")
	}
	target := others[rand.Intn(len(others))]
	if err := gc.Clan().Like(ctx, data.ClanID, target.ViewerID); err != nil {
		return err
	}
	rc.Logf("为【%s】点赞", target.Name)
	return nil
}
