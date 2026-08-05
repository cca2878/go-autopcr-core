package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
)

// 每日任务域的折叠器。库存与体力由通用层折，这里只补它们盖不到的部分。

// missionAccept 折叠任务领取带来的主公等级变化（对应参考项目 handlers.py:472）。
//
// 本响应实现了 RewardCarrier 与 StaminaCarrier，库存与体力已由通用层折走，这里只补
// team_level——它是个裸 int，没有共用模型可依附，故归域专属层。参考项目同样用
// `if self.team_level` 守卫：服务端不下发时是 0，无条件赋值会把等级抹掉。
func missionAccept(s *gamestate.PlayerState, _ protocol.Request, resp any) {
	if r := resp.(*daily.MissionAcceptResponse); r.TeamLevel > 0 {
		s.TeamLevel = r.TeamLevel
	}
}

// missionIndex 折叠任务完成状态（对应参考项目 handlers.py:461 的 mgr.missions = self.missions）。
func missionIndex(s *gamestate.PlayerState, _ protocol.Request, resp any) {
	r := resp.(*daily.MissionIndexResponse)
	m := make(map[int]int, len(r.Missions))
	for _, it := range r.Missions {
		m[it.MissionID] = it.MissionStatus
	}
	s.Missions = m
}
