// Package exequip 是母数据「EX 装备」域的只读查询（稀有度等；对应 ref ex_equipment_data）。
package exequip

import (
	"context"

	"github.com/cca2878/go-autopcr/internal/client/masterdata/mddb"
)

// API 是 EX 装备域查询契约（随功能在本包内累加）。
type API interface {
	// RarityByID 返回全部 EX 装备的 ex_equipment_id→rarity 映射（供按稀有度计数）。
	RarityByID(ctx context.Context) (map[int]int, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造 EX 装备域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) RarityByID(ctx context.Context) (map[int]int, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT ex_equipment_id, rarity FROM ex_equipment_data")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int]int)
	for rows.Next() {
		var id, rarity int
		if err := rows.Scan(&id, &rarity); err != nil {
			return nil, err
		}
		out[id] = rarity
	}
	return out, rows.Err()
}
