// Package dungeon 是母数据「地下城」域的只读查询（区域名等；对应 ref db.dungeon_name）。
package dungeon

import (
	"context"
	"database/sql"

	"github.com/cca2878/go-autopcr/internal/client/masterdata/mddb"
)

// API 是地下城域查询契约（随功能在本包内累加）。
type API interface {
	// AreaName 按 dungeon_area_id 返回区域名；不存在返回空串。
	AreaName(ctx context.Context, areaID int) (string, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造地下城域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) AreaName(ctx context.Context, areaID int) (string, error) {
	var name string
	err := a.db.QueryRowContext(ctx, "SELECT dungeon_name FROM dungeon_area_data WHERE dungeon_area_id = ?", areaID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return name, nil
}
