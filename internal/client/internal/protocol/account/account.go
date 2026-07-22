// Package account 是「账号/首页」域的 DTO（load/index；后续 profile 等）。
package account

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var urlLoadIndex = protocol.MustRelURL("load/index")

// LoadIndexRequest 拉取玩家首页索引数据。
type LoadIndexRequest struct {
	protocol.RequestBase
	Carrier string `msgpack:"carrier" json:"carrier"`
}

func (*LoadIndexRequest) URL() *url.URL { return urlLoadIndex }

// UserInfo 是玩家基础信息（此处为所需的部分字段）。
type UserInfo struct {
	ViewerID    int64  `msgpack:"viewer_id" json:"viewer_id"`
	UserName    string `msgpack:"user_name" json:"user_name"`
	TeamLevel   int    `msgpack:"team_level" json:"team_level"`
	UserStamina int    `msgpack:"user_stamina" json:"user_stamina"`
}

// UserJewel 是钻石信息。jewel 为总量，free_jewel 为免费部分。
type UserJewel struct {
	Jewel     int `msgpack:"jewel" json:"jewel"`
	FreeJewel int `msgpack:"free_jewel" json:"free_jewel"`
}

// UserGold 是金币信息（付费/免费两部分）。定义在协议基包，各域共享。
type UserGold = protocol.UserGold

// UserClan 是玩家所属公会信息（此处仅取所属公会 id）。未加入公会时该字段为空。
type UserClan struct {
	ClanID int64 `msgpack:"clan_id" json:"clan_id"`
}

// UserChara 是玩家某角色的好感信息（好感等级用于剧情解锁判定；chara_love＝累计亲密度，
// 用于喂蛋糕判定亲密度是否已满）。
type UserChara struct {
	CharaID   int `msgpack:"chara_id" json:"chara_id"`
	LoveLevel int `msgpack:"love_level" json:"love_level"`
	CharaLove int `msgpack:"chara_love" json:"chara_love"`
}

// CharaFortune 是今日赛马（chara_fortune）待抽取信息；load/index 下发非空即今日尚未赛马。
type CharaFortune struct {
	FortuneID int   `msgpack:"fortune_id" json:"fortune_id"`
	Rank      int   `msgpack:"rank" json:"rank"`
	UnitList  []int `msgpack:"unit_list" json:"unit_list"`
}

// UnitData 是玩家持有的一个角色（此处仅取练度报告所需字段）。
type UnitData struct {
	ID             int `msgpack:"id" json:"id"`
	UnitRarity     int `msgpack:"unit_rarity" json:"unit_rarity"`
	UnitLevel      int `msgpack:"unit_level" json:"unit_level"`
	PromotionLevel int `msgpack:"promotion_level" json:"promotion_level"`
}

// ExtraEquipSubStatus 是一件 EX 装备的一条副属性（对应 ref ExtraEquipSubStatus）。
// status＝属性类型(eParamType)，step＝档位(1..5，5＝满)，is_lock＝是否锁定。
type ExtraEquipSubStatus struct {
	SlotNumber int  `msgpack:"slot_number" json:"slot_number"`
	Status     int  `msgpack:"status" json:"status"`
	Step       int  `msgpack:"step" json:"step"`
	IsLock     bool `msgpack:"is_lock" json:"is_lock"`
}

// ExtraEquipInfo 是玩家持有的一件 EX 装备（对应 ref ExtraEquipInfo）。彩装炼成/战力搭配需要
// 完整实例（serial_id/rank/enhancement_pt/sub_status…），故此处取全字段而非仅 id。
type ExtraEquipInfo struct {
	SerialID       int                   `msgpack:"serial_id" json:"serial_id"`
	ExEquipmentID  int                   `msgpack:"ex_equipment_id" json:"ex_equipment_id"`
	EnhancementPt  int                   `msgpack:"enhancement_pt" json:"enhancement_pt"`
	Rank           int                   `msgpack:"rank" json:"rank"`
	ProtectionFlag int                   `msgpack:"protection_flag" json:"protection_flag"`
	SubStatus      []ExtraEquipSubStatus `msgpack:"sub_status" json:"sub_status"`
	IsAlcesPending int                   `msgpack:"is_alces_pending" json:"is_alces_pending"`
}

// InventoryInfo 是一条库存条目（对应 ref InventoryInfo，此处仅取折叠库存所需字段）。
// type＝eInventoryType，id＝物品 id，stock＝持有量（普通物品的库存以 stock 计）。
type InventoryInfo struct {
	Type  int `msgpack:"type" json:"type"`
	ID    int `msgpack:"id" json:"id"`
	Stock int `msgpack:"stock" json:"stock"`
}

// LoadIndexResponse 为所需字段的部分定义（玩家档案相关）。
type LoadIndexResponse struct {
	protocol.ResponseBase
	UserInfo       *UserInfo        `msgpack:"user_info" json:"user_info"`
	UserJewel      *UserJewel       `msgpack:"user_jewel" json:"user_jewel"`
	UserGold       *UserGold        `msgpack:"user_gold" json:"user_gold"`
	UserClan       *UserClan        `msgpack:"user_clan" json:"user_clan"`
	ClanLikeCount  int              `msgpack:"clan_like_count" json:"clan_like_count"`
	ReadStoryIDs   []int            `msgpack:"read_story_ids" json:"read_story_ids"`
	UserCharaInfo  []UserChara      `msgpack:"user_chara_info" json:"user_chara_info"`
	UnitList       []UnitData       `msgpack:"unit_list" json:"unit_list"`
	CF             *CharaFortune    `msgpack:"cf" json:"cf"`
	UserExEquip    []ExtraEquipInfo `msgpack:"user_ex_equip" json:"user_ex_equip"`
	MaterialList   []InventoryInfo  `msgpack:"material_list" json:"material_list"`
	ItemList       []InventoryInfo  `msgpack:"item_list" json:"item_list"`
	DailyResetTime int64            `msgpack:"daily_reset_time" json:"daily_reset_time"`
}
