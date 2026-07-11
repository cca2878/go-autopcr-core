// Package emblem 是母数据「称号」域的只读查询（全称号列表 + 说明；对应 ref missing_emblem）。
package emblem

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// Emblem 是一个称号（图鉴条目）：id、名称、达成说明（取自关联的 emblem_mission_data.description）。
type Emblem struct {
	ID          int
	Name        string
	Description string // 达成条件说明；无关联任务时为空
}

// API 是称号域查询契约（随功能在本包内累加）。
type API interface {
	// AllEmblems 返回全部称号（按 emblem_id 升序），供图鉴缺口判定。
	AllEmblems(ctx context.Context) ([]Emblem, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造称号域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) AllEmblems(ctx context.Context) ([]Emblem, error) {
	rows, err := a.db.QueryContext(ctx,
		"SELECT e.emblem_id, e.emblem_name, COALESCE(m.description, '') "+
			"FROM emblem_data e LEFT JOIN emblem_mission_data m ON m.mission_id = e.description_mission_id "+
			"ORDER BY e.emblem_id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Emblem
	for rows.Next() {
		var e Emblem
		if err := rows.Scan(&e.ID, &e.Name, &e.Description); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
