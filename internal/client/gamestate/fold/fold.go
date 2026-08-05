// Package fold 装的是"响应→玩家状态"的全部折叠逻辑，按游戏功能域分文件。
//
// 与 gamestate 分包的理由是依赖方向：折叠器要引用 gamestate.PlayerState，若默认注册表也放在
// gamestate 里，两边就得互相 import。故 gamestate 只留状态与注册表机制，本包持有折叠逻辑并
// 负责装配——依赖单向，职责也分得干净。
//
// 折叠面分两层：
//
//	通用层（common.go）  不认响应类型，只认它实现了哪些 Carrier 接口。参考项目的 138 个折叠
//	                    handler 里有 108 个（78%）没有域专属逻辑，全被这一层吃掉。
//	域专属层（其余文件）  真正需要手写的那 22%，按域分文件。
package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
)

// DefaultRegistry 装配通用层，并登记全部域专属折叠器。
//
// 这里是全项目唯一能一眼看出"哪些响应会改状态"的地方——通用层那部分除外，它按接口生效，
// 见各响应上的 InventoryChanges / StaminaSnapshot / GoldSnapshot / JewelSnapshot 实现。
func DefaultRegistry() *gamestate.Registry {
	r := gamestate.NewRegistry()
	r.UseCommon(common)
	r.Register((*account.LoadIndexResponse)(nil), loadIndex)
	r.Register((*account.HomeIndexResponse)(nil), homeIndex)
	r.Register((*sdk.SourceIniGetMaintenanceStatusResponse)(nil), maintenance)
	r.Register((*clan.ClanInfoResponse)(nil), clanInfo)
	r.Register((*clan.ClanLikeResponse)(nil), clanLike)
	r.Register((*daily.MissionIndexResponse)(nil), missionIndex)
	r.Register((*daily.MissionAcceptResponse)(nil), missionAccept)
	r.Register((*race.CharaFortuneDrawResponse)(nil), charaFortuneDraw)
	r.Register((*alces.ExecResponse)(nil), alcesExec)
	r.Register((*alces.FixResultResponse)(nil), alcesFixResult)
	r.Register((*alces.LockSlotResponse)(nil), alcesLockSlot)
	return r
}
