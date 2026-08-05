package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
)

// 公会域的折叠器。
// 「今日已做过」类守卫的折叠器。这些响应本身不带状态，折的是【动作已发生】这一事实——
// 不折则同一会话里重跑同一模块会绕过守卫、撞上业务错误码。
// clanLike 记下「今日已点赞」（对应 ref ClanLikeResponse 的 clan_like_count = 1）。
// 不折叠则同一会话里第二次跑点赞模块会绕过守卫、撞上业务错误码。
func clanLike(s *gamestate.PlayerState, _ protocol.Request, _ any) {
	s.ClanLikeCount = 1
}

// clanInfo 折叠公会 id（对应 ref handlers.py:865 的 mgr.clan = self.clan.detail.clan_id）。
// 与 load/index 折的是同一个 ClanID，保持同源：哪个响应回传就以哪个为准，不必等下次登录。
func clanInfo(s *gamestate.PlayerState, _ protocol.Request, resp any) {
	if r := resp.(*clan.ClanInfoResponse); r.Clan != nil && r.Clan.Detail != nil {
		s.ClanID = r.Clan.Detail.ClanID
	}
}
