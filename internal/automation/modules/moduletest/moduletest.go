// Package moduletest 提供模块单测共用的可配置假 GameClient 与运行助手。
//
// 各域模块子包（modules/daily、modules/story…）的 _test.go 用 FakeClient 覆写它需要的访问器/
// 状态，再用各自域的 fake 能力面注入，从而复用两层 fake 模式而不必每个子包重写一遍。
package moduletest

import (
	"context"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/account"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/arena"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/clan"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/clanbattle"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/daily"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/dungeon"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/emblem"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/mirage"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/race"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/room"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/seasonpass"
	"github.com/cca2878/go-autopcr/internal/client/gameapi/tower"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
)

// FakeClient 是可配置的假 GameClient：内嵌真接口（nil），按需覆写各访问器与状态字段。
// 未设置的字段其访问器返回零值（内嵌 nil 接口调用会 panic，故只覆写被测模块用到的部分）。
type FakeClient struct {
	client.GameClient
	State         *gamestate.PlayerState
	ServerTimeVal int64
	MD            masterdata.Reader
	AccountAPI    account.API
	DailyAPI      daily.API
	RoomAPI       room.API
	ClanAPI       clan.API
	ArenaAPI      arena.API
	RaceAPI       race.API
	EmblemAPI     emblem.API
	DungeonAPI    dungeon.API
	SeasonpassAPI seasonpass.API
	ClanBattleAPI clanbattle.API
	MirageAPI     mirage.API
	TowerAPI      tower.API
}

func (f *FakeClient) Data() *gamestate.PlayerState  { return f.State }
func (f *FakeClient) ServerTime() int64             { return f.ServerTimeVal }
func (f *FakeClient) Masterdata() masterdata.Reader { return f.MD }
func (f *FakeClient) Account() account.API          { return f.AccountAPI }
func (f *FakeClient) Daily() daily.API              { return f.DailyAPI }
func (f *FakeClient) Room() room.API                { return f.RoomAPI }
func (f *FakeClient) Clan() clan.API                { return f.ClanAPI }
func (f *FakeClient) Arena() arena.API              { return f.ArenaAPI }
func (f *FakeClient) Race() race.API                { return f.RaceAPI }
func (f *FakeClient) Emblem() emblem.API            { return f.EmblemAPI }
func (f *FakeClient) Dungeon() dungeon.API          { return f.DungeonAPI }
func (f *FakeClient) Seasonpass() seasonpass.API    { return f.SeasonpassAPI }
func (f *FakeClient) ClanBattle() clanbattle.API    { return f.ClanBattleAPI }
func (f *FakeClient) Mirage() mirage.API            { return f.MirageAPI }
func (f *FakeClient) Tower() tower.API              { return f.TowerAPI }

// RunOne 用给定模块与假客户端跑单任务并返回结果（经真实 Runner，覆盖校验/隔离逻辑）。
func RunOne(gc client.GameClient, m automation.Module, values map[string]any) automation.Result {
	reg := automation.NewRegistry()
	reg.Register(m)
	return automation.Run(context.Background(), gc, reg, []automation.Task{{Module: m.Meta().Name, Values: values}})[0]
}
