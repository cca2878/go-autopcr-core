// Package schedule 是母数据"活动日程"域的只读查询（各系统开放时段汇总；对应参考项目 half_schedule）。
//
// 半月刊需汇总多张排程表（公会战/女神祭/扭蛋/庆典/活动/露娜塔…）。本域用统一的 Entry 归一各表，
// 顶层聚合。当前收录一组高信号排程表，其余同类表按同法追加即可（每张表一条 collect 调用）。
package schedule

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// Entry 是一条排程（某系统的一段开放时段）。
type Entry struct {
	Start  time.Time
	End    time.Time
	Label  string // 系统名（公会战/女神祭…）
	Detail string // 附加说明（活动名等，可空）
}

// API 是活动日程域查询契约（随功能在本包内累加）。
type API interface {
	// Schedules 返回收录的全部排程（未排序；调用方自行筛选/排序）。
	Schedules(ctx context.Context) ([]Entry, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造活动日程域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) Schedules(ctx context.Context) ([]Entry, error) {
	var out []Entry
	// 无名排程表：(start_time, end_time) → 固定 label。
	plain := []struct{ query, label string }{
		{"SELECT start_time, end_time FROM clan_battle_period", "公会战"},
		{"SELECT start_time, end_time FROM hatsune_schedule", "活动"},
		{"SELECT start_time, end_time FROM tower_schedule", "露娜塔"},
		{"SELECT start_time, end_time FROM campaign_schedule", "庆典"},
	}
	for _, p := range plain {
		if err := a.collect(ctx, p.query, p.label, false, &out); err != nil {
			return nil, err
		}
	}
	// 带名排程表：(start_time, end_time, name) → label + 名称。
	if err := a.collect(ctx, "SELECT start_time, end_time, name FROM seasonpass_foundation", "女神祭", true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// collect 执行一条排程查询并把结果并入 out；hasName 时第三列为附加说明。无法解析时间的行跳过。
func (a *Impl) collect(ctx context.Context, query, label string, hasName bool, out *[]Entry) error {
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var startStr, endStr, name string
		if hasName {
			if err := rows.Scan(&startStr, &endStr, &name); err != nil {
				return err
			}
		} else {
			if err := rows.Scan(&startStr, &endStr); err != nil {
				return err
			}
		}
		start, err := mddb.ParseTime(startStr)
		if err != nil {
			continue
		}
		end, err := mddb.ParseTime(endStr)
		if err != nil {
			continue
		}
		*out = append(*out, Entry{Start: start, End: end, Label: label, Detail: name})
	}
	return rows.Err()
}
