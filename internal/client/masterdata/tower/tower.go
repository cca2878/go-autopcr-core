// Package tower 是母数据「露娜塔」域的只读查询（最新一期开放窗口；对应 ref get_newest_tower_id）。
package tower

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// API 是露娜塔域查询契约（随功能在本包内累加）。
type API interface {
	// NewestWindow 返回最新一期露娜塔的开放窗口（按 start_time 最大者）；无数据时 ok=false。
	NewestWindow(ctx context.Context) (start, end time.Time, ok bool, err error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造露娜塔域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) NewestWindow(ctx context.Context) (time.Time, time.Time, bool, error) {
	// start_time 为字符串（格式混杂），故取全部在 Go 侧解析比较，取 start 最大的一期。
	rows, err := a.db.QueryContext(ctx, "SELECT start_time, end_time FROM tower_schedule")
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	defer func() { _ = rows.Close() }()
	var (
		bestStart, bestEnd time.Time
		found              bool
	)
	for rows.Next() {
		var startStr, endStr string
		if err := rows.Scan(&startStr, &endStr); err != nil {
			return time.Time{}, time.Time{}, false, err
		}
		start, err := mddb.ParseTime(startStr)
		if err != nil {
			continue
		}
		end, err := mddb.ParseTime(endStr)
		if err != nil {
			continue
		}
		if !found || start.After(bestStart) {
			bestStart, bestEnd, found = start, end, true
		}
	}
	return bestStart, bestEnd, found, rows.Err()
}
