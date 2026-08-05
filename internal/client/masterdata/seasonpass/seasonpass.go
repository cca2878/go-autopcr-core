// Package seasonpass 是母数据"女神祭（季卡）"域的只读查询（进行中的女神祭；对应参考项目 get_active_seasonpass）。
package seasonpass

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// API 是女神祭域查询契约（随功能在本包内累加）。
type API interface {
	// ActiveSeasonIDs 返回 now 处于 [start_time, limit_time] 内的女神祭 season_id 列表
	// （对应参考项目 get_active_seasonpass：可领任务的开放窗口）。
	ActiveSeasonIDs(ctx context.Context, now time.Time) ([]int, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造女神祭域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) ActiveSeasonIDs(ctx context.Context, now time.Time) ([]int, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT season_id, start_time, limit_time FROM seasonpass_foundation")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var (
			id               int
			startStr, limStr string
		)
		if err := rows.Scan(&id, &startStr, &limStr); err != nil {
			return nil, err
		}
		start, err := mddb.ParseTime(startStr)
		if err != nil {
			continue
		}
		lim, err := mddb.ParseTime(limStr)
		if err != nil {
			continue
		}
		if !now.Before(start) && !now.After(lim) {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}
