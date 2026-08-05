package gamestate

import (
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
)

// 「今日已做过」类守卫的折叠器。这些响应本身不带状态，折的是【动作已发生】这一事实——
// 不折则同一会话里重跑同一模块会绕过守卫、撞上业务错误码。
// foldClanLike 记下「今日已点赞」（对应 ref ClanLikeResponse 的 clan_like_count = 1）。
// 不折叠则同一会话里第二次跑点赞模块会绕过守卫、撞上业务错误码。
func foldClanLike(s *PlayerState, _ protocol.Request, _ any) {
	s.ClanLikeCount = 1
}

// foldCharaFortuneDraw 抽完即清空今日赛马待抽信息（对应 ref daily.py 的 client.data.cf = None）。
// 同理：不清则同一会话里重跑赛马模块会绕过「今日已赛马」守卫、重复发抽取请求。
func foldCharaFortuneDraw(s *PlayerState, _ protocol.Request, _ any) {
	s.CharaFortune = nil
}

// foldMissionAccept 折叠任务领取带来的主公等级变化（对应 ref handlers.py:472）。
//
// 库存与体力由通用层折（本响应实现了 RewardCarrier 与 StaminaCarrier），这里只补 team_level
// ——它是个裸 int，没有共用模型可依附，故归域专属层。ref 同样用 `if self.team_level` 守卫：
// 服务端不下发时是 0，无条件赋值会把等级抹掉。
func foldMissionAccept(s *PlayerState, _ protocol.Request, resp any) {
	if r := resp.(*daily.MissionAcceptResponse); r.TeamLevel > 0 {
		s.TeamLevel = r.TeamLevel
	}
}

// foldClanInfo 折叠公会 id（对应 ref handlers.py:865 的 mgr.clan = self.clan.detail.clan_id）。
// 与 load/index 折的是同一个 ClanID，保持同源：哪个响应回传就以哪个为准，不必等下次登录。
func foldClanInfo(s *PlayerState, _ protocol.Request, resp any) {
	if r := resp.(*clan.ClanInfoResponse); r.Clan != nil && r.Clan.Detail != nil {
		s.ClanID = r.Clan.Detail.ClanID
	}
}

// foldMissionIndex 折叠任务完成状态（对应 ref handlers.py:461 的 mgr.missions = self.missions）。
func foldMissionIndex(s *PlayerState, _ protocol.Request, resp any) {
	r := resp.(*daily.MissionIndexResponse)
	m := make(map[int]int, len(r.Missions))
	for _, it := range r.Missions {
		m[it.MissionID] = it.MissionStatus
	}
	s.Missions = m
}
