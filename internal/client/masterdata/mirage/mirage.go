// Package mirage 是母数据「追忆战」域的只读查询（礼物池累积上限；对应 ref get_mirage_setting）。
package mirage

import (
	"context"
	"database/sql"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// API 是追忆战域查询契约（随功能在本包内累加）。
type API interface {
	// AccumulateDayMax 返回礼物池累积天数上限（对应 ref get_mirage_setting 里最新一行的
	// pool_reward_accumulate_day_num_max）；无数据返回 0。
	AccumulateDayMax(ctx context.Context) (int, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造追忆战域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) AccumulateDayMax(ctx context.Context) (int, error) {
	var days int
	// ref 取 id 最大的一行；此处等价地取按 id 降序的首行。
	err := a.db.QueryRowContext(ctx,
		"SELECT pool_reward_accumulate_day_num_max FROM mirage_setting ORDER BY id DESC LIMIT 1").Scan(&days)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return days, nil
}
