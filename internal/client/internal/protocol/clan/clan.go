// Package clan 是"公会"域的 DTO（公会信息、点赞等；对应参考项目 clan.py）。
package clan

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlClanInfo = protocol.MustRelURL("clan/info")
	urlClanLike = protocol.MustRelURL("clan/like")
)

// ClanInfoRequest 拉取指定公会的信息（成员列表等）。get_user_equip=0 不带装备明细。
type ClanInfoRequest struct {
	protocol.RequestBase
	ClanID       int64 `msgpack:"clan_id" json:"clan_id"`
	GetUserEquip int   `msgpack:"get_user_equip" json:"get_user_equip"`
}

func (*ClanInfoRequest) URL() *url.URL { return urlClanInfo }

// ClanMemberInfo 是公会一名成员（此处仅取点赞所需字段）。
type ClanMemberInfo struct {
	ViewerID int64  `msgpack:"viewer_id" json:"viewer_id"`
	Name     string `msgpack:"name" json:"name"`
}

// ClanData 是公会信息主体（此处仅取成员列表）。
// ClanDetail 是公会详情（此处仅取公会 id）。
type ClanDetail struct {
	ClanID int64 `msgpack:"clan_id" json:"clan_id"`
}

type ClanData struct {
	Detail  *ClanDetail      `msgpack:"detail" json:"detail"`
	Members []ClanMemberInfo `msgpack:"members" json:"members"`
}

// ClanInfoResponse 携带公会信息（其余字段由解码器忽略）。
type ClanInfoResponse struct {
	protocol.ResponseBase
	Clan *ClanData `msgpack:"clan" json:"clan"`
}

// ClanLikeRequest 为指定成员点赞。
type ClanLikeRequest struct {
	protocol.RequestBase
	ClanID         int64 `msgpack:"clan_id" json:"clan_id"`
	TargetViewerID int64 `msgpack:"target_viewer_id" json:"target_viewer_id"`
}

func (*ClanLikeRequest) URL() *url.URL { return urlClanLike }

// ClanLikeResponse 为点赞结果（无需读取的字段由解码器忽略）。
type ClanLikeResponse struct {
	StaminaInfo *protocol.UserStaminaInfo `msgpack:"stamina_info" json:"stamina_info"`
	protocol.ResponseBase
}

// StaminaSnapshot 实现 protocol.StaminaCarrier（对应参考项目 handlers.py 的 ClanLikeResponse）。
func (r *ClanLikeResponse) StaminaSnapshot() *protocol.UserStaminaInfo { return r.StaminaInfo }
