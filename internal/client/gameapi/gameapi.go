// Package gameapi 是无头客户端对外暴露的「游戏 API 能力面」——供自动化模块(S3)按参数调用。
//
// 能力**按游戏功能域拆分为子包**（account、daily、room、clan…），每子包定义该域的接口 +
// 实现 + 结果类型；GameAPI 以**访问器**形式聚合各域（gc.Account()/gc.Daily()…），从源头避免
// 长成一个巨型对象。每个方法内部完成请求体构造、发包、解码，调用方无需接触 protocol（仍隐藏
// 在 internal/client/internal 下，本子包在客户端子树内故可引用）。
package gameapi

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/account"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/arena"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/clanbattle"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/dungeon"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/emblem"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/mirage"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/race"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/room"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/seasonpass"
	"github.com/cca2878/go-autopcr-core/internal/client/gameapi/tower"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// GameAPI 以访问器聚合各功能域能力面。新增域时在此加一个访问器方法。
type GameAPI interface {
	// Account 返回账号/首页域能力面。
	Account() account.API
	// Daily 返回每日收取域能力面（礼物箱、任务奖励…）。
	Daily() daily.API
	// Room 返回家园域能力面（家园产物收取…）。
	Room() room.API
	// Clan 返回公会域能力面（公会信息、点赞…）。
	Clan() clan.API
	// Arena 返回竞技场域能力面（jjc/pjjc 时间奖励领取…）。
	Arena() arena.API
	// Race 返回赛马域能力面（chara_fortune 抽取…）。
	Race() race.API
	// Emblem 返回称号域能力面（emblem/top 已拥有称号…）。
	Emblem() emblem.API
	// Dungeon 返回地下城域能力面（dungeon/info 状态…）。
	Dungeon() dungeon.API
	// Seasonpass 返回女神祭域能力面（任务领取…）。
	Seasonpass() seasonpass.API
	// ClanBattle 返回公会战域能力面（clan_battle/top 刀数…）。
	ClanBattle() clanbattle.API
	// Mirage 返回追忆战域能力面（礼物池领取…）。
	Mirage() mirage.API
	// Tower 返回露娜塔域能力面（tower/top 回廊状态…）。
	Tower() tower.API
}

// set 是 GameAPI 的实现：持有各域实现，经访问器暴露。
type set struct {
	account    *account.Impl
	daily      *daily.Impl
	room       *room.Impl
	clan       *clan.Impl
	arena      *arena.Impl
	race       *race.Impl
	emblem     *emblem.Impl
	dungeon    *dungeon.Impl
	seasonpass *seasonpass.Impl
	clanbattle *clanbattle.Impl
	mirage     *mirage.Impl
	tower      *tower.Impl
}

// New 用传输句柄装配全部域能力面。
func New(tr *transport.Client) GameAPI {
	return &set{
		account:    account.New(tr),
		daily:      daily.New(tr),
		room:       room.New(tr),
		clan:       clan.New(tr),
		arena:      arena.New(tr),
		race:       race.New(tr),
		emblem:     emblem.New(tr),
		dungeon:    dungeon.New(tr),
		seasonpass: seasonpass.New(tr),
		clanbattle: clanbattle.New(tr),
		mirage:     mirage.New(tr),
		tower:      tower.New(tr),
	}
}

func (s *set) Account() account.API       { return s.account }
func (s *set) Daily() daily.API           { return s.daily }
func (s *set) Room() room.API             { return s.room }
func (s *set) Clan() clan.API             { return s.clan }
func (s *set) Arena() arena.API           { return s.arena }
func (s *set) Race() race.API             { return s.race }
func (s *set) Emblem() emblem.API         { return s.emblem }
func (s *set) Dungeon() dungeon.API       { return s.dungeon }
func (s *set) Seasonpass() seasonpass.API { return s.seasonpass }
func (s *set) ClanBattle() clanbattle.API { return s.clanbattle }
func (s *set) Mirage() mirage.API         { return s.mirage }
func (s *set) Tower() tower.API           { return s.tower }
