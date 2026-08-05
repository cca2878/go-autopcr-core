package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// 赛马域的折叠器。
// charaFortuneDraw 抽完即清空今日赛马待抽信息（对应 ref daily.py 的 client.data.cf = None）。
// 同理：不清则同一会话里重跑赛马模块会绕过「今日已赛马」守卫、重复发抽取请求。
func charaFortuneDraw(s *gamestate.PlayerState, _ protocol.Request, _ any) {
	s.CharaFortune = nil
}
