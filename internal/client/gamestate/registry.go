package gamestate

import (
	"reflect"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// Folder 把一次「请求-响应」折叠进 PlayerState。resp 为 *R（具体响应类型的指针）。
//
// 之所以连 req 一并传入：服务端有一整类端点只回结果、不回「改的是哪个键」，不看请求就无从
// 定位该更新哪条状态。ref 的 138 个 handler 里有 20 个属于此类，且形态一致——
// story/viewing 不回传读了哪篇（靠 request.story_id）、quest/skip 只回 daily_clear_count
// 不回 quest_id、deck/update 干脆整份编队都从请求里抄。这是协议的固有性质，不是某种实现偏好，
// 故签名一开始就留出 req，免得将来为它翻修全部折叠器。
type Folder func(s *PlayerState, req protocol.Request, resp any)

// CommonFolder 是通用层折叠器：不认响应的具体类型，只认它实现了哪些 Carrier 接口，
// 因而对每个响应都跑一遍，无需登记（见 gamestate/fold 包）。
type CommonFolder func(s *PlayerState, resp any)

// Registry 把响应分发给折叠器（对应原项目 handlers 的注册表）。
//
// 它只是机制，不含任何折叠逻辑——具体折叠器住在子包 gamestate/fold 里。这样拆是因为折叠器
// 要引用本包的 PlayerState，若把注册表的默认装配也放在本包，两边就会互相 import。
type Registry struct {
	folders map[reflect.Type]Folder
	common  []CommonFolder
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{folders: make(map[reflect.Type]Folder)}
}

// Register 为 sample 的具体类型登记一个 Folder。sample 传 (*R)(nil) 即可。
func (r *Registry) Register(sample any, f Folder) {
	r.folders[reflect.TypeOf(sample)] = f
}

// UseCommon 安装一个通用层折叠器，对每个响应都跑。
func (r *Registry) UseCommon(f CommonFolder) {
	r.common = append(r.common, f)
}

// Apply 把一次「请求-响应」折叠进 s：先跑通用层（响应实现了哪个 Carrier 就折哪部分，
// 无需登记），再跑该响应类型登记的域专属折叠器；两层都没有则忽略。
//
// 专属在后，是为了让它能覆盖通用层的结果——同一份数据若两层都碰，更精确的那个说了算。
func (r *Registry) Apply(s *PlayerState, req protocol.Request, resp any) {
	for _, f := range r.common {
		f(s, resp)
	}
	if f, ok := r.folders[reflect.TypeOf(resp)]; ok {
		f(s, req, resp)
	}
}
