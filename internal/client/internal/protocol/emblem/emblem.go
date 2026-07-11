// Package emblem 是「称号」域的 DTO（emblem/top；对应 ref missing_emblem）。
package emblem

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var urlEmblemTop = protocol.MustRelURL("emblem/top")

// EmblemTopRequest 拉取称号总览（含已拥有称号列表）。
type EmblemTopRequest struct {
	protocol.RequestBase
}

func (*EmblemTopRequest) URL() *url.URL { return urlEmblemTop }

// UserEmblem 是玩家已拥有的一个称号（此处仅取 emblem_id）。
type UserEmblem struct {
	EmblemID int `msgpack:"emblem_id" json:"emblem_id"`
}

// EmblemTopResponse 携带已拥有称号列表（其余字段由解码器忽略）。
type EmblemTopResponse struct {
	protocol.ResponseBase
	UserEmblemList []UserEmblem `msgpack:"user_emblem_list" json:"user_emblem_list"`
}
