// Package unit 是母数据「角色」域的只读查询（角色名、数量等；随需增量）。
package unit

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// Obtainable 是一个可获得角色（图鉴条目）：id、名称、是否限定。
type Obtainable struct {
	UnitID    int
	Name      string
	IsLimited bool
}

// API 是角色域查询契约（随功能在本包内累加）。
type API interface {
	// Name 按 unit_id 返回角色名；不存在返回 sql.ErrNoRows。
	Name(ctx context.Context, unitID int) (string, error)
	// Count 返回角色总数。
	Count(ctx context.Context) (int, error)
	// Obtainables 返回全部可获得角色（unlock_unit_condition ∩ unit_data），供图鉴缺口判定。
	Obtainables(ctx context.Context) ([]Obtainable, error)
	// MaxTotalLove 返回给定星级下的好感上限（love_level, total_love）——对应 ref db.max_total_love：
	// 取 rarity ≤ 给定值的排程里 total_love 最大者（喂蛋糕判定亲密度是否已满）。
	MaxTotalLove(ctx context.Context, rarity int) (loveLevel, totalLove int, err error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造角色域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) Name(ctx context.Context, unitID int) (string, error) {
	var name string
	err := a.db.QueryRowContext(ctx, "SELECT unit_name FROM unit_data WHERE unit_id = ?", unitID).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (a *Impl) Count(ctx context.Context) (int, error) {
	var n int
	if err := a.db.QueryRowContext(ctx, "SELECT count(*) FROM unit_data").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (a *Impl) Obtainables(ctx context.Context) ([]Obtainable, error) {
	rows, err := a.db.QueryContext(ctx,
		"SELECT u.unit_id, d.unit_name, d.is_limited FROM unlock_unit_condition u JOIN unit_data d ON d.unit_id = u.unit_id ORDER BY u.unit_id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Obtainable
	for rows.Next() {
		var (
			id        int
			name      string
			isLimited int
		)
		if err := rows.Scan(&id, &name, &isLimited); err != nil {
			return nil, err
		}
		out = append(out, Obtainable{UnitID: id, Name: name, IsLimited: isLimited != 0})
	}
	return out, rows.Err()
}

func (a *Impl) MaxTotalLove(ctx context.Context, rarity int) (int, int, error) {
	// love_chara 以 love_level 为主键、每行带 (total_love, rarity)。ref 按 rarity 分组取组内
	// (love_level,total_love) 最大，再对 rarity ≤ 给定值取最大；因两列随好感单调递增，等价于
	// 直接取 rarity ≤ 给定值范围内两列各自的最大值。
	var loveLevel, totalLove int
	err := a.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(love_level), 0), COALESCE(MAX(total_love), 0) FROM love_chara WHERE rarity <= ?", rarity).
		Scan(&loveLevel, &totalLove)
	if err != nil {
		return 0, 0, err
	}
	return loveLevel, totalLove, nil
}
