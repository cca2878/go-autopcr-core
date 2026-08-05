package gamestate

import (
	"reflect"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/alces"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/race"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
)

// Folder 把一次「请求-响应」折叠进 PlayerState。resp 为 *R（具体响应类型的指针）。
//
// 之所以连 req 一并传入：服务端有一整类端点只回结果、不回「改的是哪个键」，不看请求就无从
// 定位该更新哪条状态。ref 的 138 个 handler 里有 20 个属于此类，且形态一致——
// story/viewing 不回传读了哪篇（靠 request.story_id）、quest/skip 只回 daily_clear_count
// 不回 quest_id、deck/update 干脆整份编队都从请求里抄。这是协议的固有性质，不是某种实现偏好，
// 故签名一开始就留出 req，免得将来为它翻修全部折叠器。
type Folder func(s *PlayerState, req protocol.Request, resp any)

// Registry 按响应的具体类型分发到对应 Folder（对应原项目 handlers 的注册表）。
type Registry struct {
	folders map[reflect.Type]Folder
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{folders: make(map[reflect.Type]Folder)}
}

// Register 为 sample 的具体类型登记一个 Folder。sample 传 (*R)(nil) 即可。
func (r *Registry) Register(sample any, f Folder) {
	r.folders[reflect.TypeOf(sample)] = f
}

// Apply 把一次「请求-响应」折叠进 s：先跑通用层（凡实现 RewardCarrier 的响应都折其库存
// 变动，无需登记），再跑该响应类型登记的域专属折叠器；两层都没有则忽略。
//
// 专属在后，是为了让它能覆盖通用层的结果——同一份数据若两层都碰，更精确的那个说了算。
func (r *Registry) Apply(s *PlayerState, req protocol.Request, resp any) {
	foldCommon(s, resp)
	if f, ok := r.folders[reflect.TypeOf(resp)]; ok {
		f(s, req, resp)
	}
}

// DefaultRegistry 登记 M2 登录序列涉及的折叠器。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register((*account.LoadIndexResponse)(nil), foldLoadIndex)
	r.Register((*account.HomeIndexResponse)(nil), foldHomeIndex)
	r.Register((*sdk.SourceIniGetMaintenanceStatusResponse)(nil), foldMaintenance)
	r.Register((*clan.ClanInfoResponse)(nil), foldClanInfo)
	r.Register((*clan.ClanLikeResponse)(nil), foldClanLike)
	r.Register((*daily.MissionIndexResponse)(nil), foldMissionIndex)
	r.Register((*daily.MissionAcceptResponse)(nil), foldMissionAccept)
	r.Register((*race.CharaFortuneDrawResponse)(nil), foldCharaFortuneDraw)
	r.Register((*alces.ExecResponse)(nil), foldAlcesExec)
	r.Register((*alces.FixResultResponse)(nil), foldAlcesFixResult)
	r.Register((*alces.LockSlotResponse)(nil), foldAlcesLockSlot)
	return r
}
