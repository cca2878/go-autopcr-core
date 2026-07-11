// Package race 是母数据「角色赛马」域的只读查询（赛马开放时段等；对应 ref chara_fortune_schedule）。
package race

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// API 是赛马域查询契约（随功能在本包内累加）。
type API interface {
	// IsFortuneTime 报告 now 是否落在任一赛马排程的 [start,end] 内（对应 ref db.is_cf_time）。
	IsFortuneTime(ctx context.Context, now time.Time) (bool, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造赛马域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) IsFortuneTime(ctx context.Context, now time.Time) (bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT start_time, end_time FROM chara_fortune_schedule")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var startStr, endStr string
		if err := rows.Scan(&startStr, &endStr); err != nil {
			return false, err
		}
		start, err := mddb.ParseTime(startStr)
		if err != nil {
			continue
		}
		end, err := mddb.ParseTime(endStr)
		if err != nil {
			continue
		}
		if !now.Before(start) && !now.After(end) {
			return true, nil
		}
	}
	return false, rows.Err()
}
