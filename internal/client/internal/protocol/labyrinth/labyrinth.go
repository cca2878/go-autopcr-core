// Package labyrinth 是「黎明界（迷宫）」域的 DTO（labyrinth/top、enter、retire；对应 ref
// LabyrinthTop/Enter/RetireRequest/Response）。用于刷开局：enter 拉一张随机地图(map_list)，
// 不满足条件则 retire 重刷。
package labyrinth

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlTop    = protocol.MustRelURL("labyrinth/top")
	urlEnter  = protocol.MustRelURL("labyrinth/enter")
	urlRetire = protocol.MustRelURL("labyrinth/retire")
)

// MapInfo 是黎明界地图上的一个格子（对应 ref LabyrinthMapInfo）。block_type 为 eLabyrinthBlockType：
// 1 起点、2 普通怪、3 EX怪、4 角色、5 事件、6 遗物、7 商店、8 Boss。
type MapInfo struct {
	Area            int   `msgpack:"area" json:"area"`
	Column          int   `msgpack:"column" json:"column"`
	Row             int   `msgpack:"row" json:"row"`
	BlockID         int   `msgpack:"block_id" json:"block_id"`
	BlockType       int   `msgpack:"block_type" json:"block_type"`
	QuestID         int   `msgpack:"quest_id" json:"quest_id"`
	NextBlockIDList []int `msgpack:"next_block_id_list" json:"next_block_id_list"`
}

// GuildClearedDifficultyInfo 是某公会已通关的难度（对应 ref LabyrinthGuildClearedDifficultyInfo）。
type GuildClearedDifficultyInfo struct {
	GuildID    int `msgpack:"guild_id" json:"guild_id"`
	Difficulty int `msgpack:"difficulty" json:"difficulty"`
}

// TopRequest 拉取黎明界首页（当前开局与已通关难度等）。
type TopRequest struct{ protocol.RequestBase }

func (*TopRequest) URL() *url.URL { return urlTop }

// TopResponse 携带当前开局 id 与各公会已通关难度。enter_id 非 0＝已有开局。
type TopResponse struct {
	protocol.ResponseBase
	EnterID                    int                          `msgpack:"enter_id" json:"enter_id"`
	GuildClearedDifficultyList []GuildClearedDifficultyInfo `msgpack:"guild_cleared_difficulty_list" json:"guild_cleared_difficulty_list"`
}

// EnterRequest 以指定公会与难度进入黎明界（生成一张随机地图）。
type EnterRequest struct {
	protocol.RequestBase
	GuildID    int `msgpack:"guild_id" json:"guild_id"`
	Difficulty int `msgpack:"difficulty" json:"difficulty"`
}

func (*EnterRequest) URL() *url.URL { return urlEnter }

// EnterResponse 携带本局 id 与地图格子列表(map_list)。
type EnterResponse struct {
	protocol.ResponseBase
	EnterID int       `msgpack:"enter_id" json:"enter_id"`
	MapList []MapInfo `msgpack:"map_list" json:"map_list"`
}

// RetireRequest 撤退当前开局。
type RetireRequest struct {
	protocol.RequestBase
	EnterID int `msgpack:"enter_id" json:"enter_id"`
}

func (*RetireRequest) URL() *url.URL { return urlRetire }

// RetireResponse 撤退结果（本模块不需其字段）。
type RetireResponse struct {
	protocol.ResponseBase
}
