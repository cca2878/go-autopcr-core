package masterdata

import (
	"context"
	"database/sql"

	"github.com/cca2878/go-autopcr/internal/client/masterdata/dungeon"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/emblem"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/exequip"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/mddb"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/mirage"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/mission"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/race"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/schedule"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/seasonpass"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/story"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/tower"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/unit"
)

// Reader 是母数据【只读查询面】：无头客户端经 gc.Masterdata() 把它暴露给上层自动化模块。
//
// 组织（与 gameapi/protocol 一致）：查询【按游戏功能域拆子包】（masterdata/mission、unit…），
// 顶层以【访问器】聚合各域（md.Mission()/md.Unit()…），从源头避免长成巨型对象、也为日后大量
// 查询方法预留确定落位。每种查询目的在其域子包内**唯一实现**，杜绝散落重复。
//
// 另暴露【低层只读查询】作兜底：个别模块偶尔需要特有的复杂查询时可内联实现，无需每次都给某个
// 域新增方法，兼顾工程化与便捷性。返回接口而非具体 *Query 是为便于上层 mock 单测。
type Reader interface {
	// —— 各域查询访问器（新增域时在此加一个）——
	Mission() mission.API
	Story() story.API
	Unit() unit.API
	Race() race.API
	Emblem() emblem.API
	Dungeon() dungeon.API
	Exequip() exequip.API
	Seasonpass() seasonpass.API
	Schedule() schedule.API
	Mirage() mirage.API
	Tower() tower.API

	// —— 低层只读兜底（供特殊一次性查询内联实现）——
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// 编译期断言 *Query 满足只读查询面契约。
var _ Reader = (*Query)(nil)

// Query 是干净母数据库的只读查询句柄：持有低层 DB 并以访问器聚合各域查询。
type Query struct {
	db         *mddb.DB
	mission    *mission.Impl
	story      *story.Impl
	unit       *unit.Impl
	race       *race.Impl
	emblem     *emblem.Impl
	dungeon    *dungeon.Impl
	exequip    *exequip.Impl
	seasonpass *seasonpass.Impl
	schedule   *schedule.Impl
	mirage     *mirage.Impl
	tower      *tower.Impl
}

// Open 以只读模式打开 path 处的干净母数据库并装配各域查询。
func Open(path string) (*Query, error) {
	db, err := mddb.Open(path)
	if err != nil {
		return nil, err
	}
	return &Query{
		db:         db,
		mission:    mission.New(db),
		story:      story.New(db),
		unit:       unit.New(db),
		race:       race.New(db),
		emblem:     emblem.New(db),
		dungeon:    dungeon.New(db),
		exequip:    exequip.New(db),
		seasonpass: seasonpass.New(db),
		schedule:   schedule.New(db),
		mirage:     mirage.New(db),
		tower:      tower.New(db),
	}, nil
}

func (q *Query) Mission() mission.API       { return q.mission }
func (q *Query) Story() story.API           { return q.story }
func (q *Query) Unit() unit.API             { return q.unit }
func (q *Query) Race() race.API             { return q.race }
func (q *Query) Emblem() emblem.API         { return q.emblem }
func (q *Query) Dungeon() dungeon.API       { return q.dungeon }
func (q *Query) Exequip() exequip.API       { return q.exequip }
func (q *Query) Seasonpass() seasonpass.API { return q.seasonpass }
func (q *Query) Schedule() schedule.API     { return q.schedule }
func (q *Query) Mirage() mirage.API         { return q.mirage }
func (q *Query) Tower() tower.API           { return q.tower }

// QueryContext 执行只读多行查询（低层兜底；调用方负责 Close 返回的 *sql.Rows）。
func (q *Query) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}

// QueryRowContext 执行只读单行查询（低层兜底）。
func (q *Query) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

// Close 释放底层连接。
func (q *Query) Close() error {
	if q == nil {
		return nil
	}
	return q.db.Close()
}
