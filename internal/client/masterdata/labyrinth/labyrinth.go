// Package labyrinth 是母数据「黎明界（迷宫）」域的只读查询（Boss 解析等；对应 ref
// labyrinth_quest_data / labyrinth_wave_group_data / labyrinth_enemy_parameter）。
package labyrinth

import (
	"context"
	"sort"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// API 是黎明界域查询契约（随功能在本包内累加）。
type API interface {
	// BossUnitIDsByQuest 返回 quest_id → 该关卡波次内敌方 unit_id 列表（去重升序），供按 quest 解析
	// Boss 单位（对应 ref _boss_unit_ids：quest→wave_group→enemy→unit_id）。
	BossUnitIDsByQuest(ctx context.Context) (map[int][]int, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造黎明界域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

// scanner 抽象 *sql.Rows 的 Scan。
type scanner interface{ Scan(dest ...any) error }

func (a *Impl) scan(ctx context.Context, query string, fn func(scanner) error) error {
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (a *Impl) BossUnitIDsByQuest(ctx context.Context) (map[int][]int, error) {
	// quest_id → wave_group_id
	questWave := map[int]int{}
	if err := a.scan(ctx, "SELECT quest_id, wave_group_id FROM labyrinth_quest_data", func(r scanner) error {
		var q, w int
		if err := r.Scan(&q, &w); err != nil {
			return err
		}
		questWave[q] = w
		return nil
	}); err != nil {
		return nil, err
	}

	// wave_group_id → [enemy_id...]
	waveEnemies := map[int][]int{}
	if err := a.scan(ctx, "SELECT wave_group_id, enemy_id_1, enemy_id_2, enemy_id_3, enemy_id_4, enemy_id_5 FROM labyrinth_wave_group_data", func(r scanner) error {
		var w int
		var e [5]int
		if err := r.Scan(&w, &e[0], &e[1], &e[2], &e[3], &e[4]); err != nil {
			return err
		}
		for _, eid := range e {
			if eid != 0 {
				waveEnemies[w] = append(waveEnemies[w], eid)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// enemy_id → unit_id
	enemyUnit := map[int]int{}
	if err := a.scan(ctx, "SELECT enemy_id, unit_id FROM labyrinth_enemy_parameter", func(r scanner) error {
		var e, u int
		if err := r.Scan(&e, &u); err != nil {
			return err
		}
		enemyUnit[e] = u
		return nil
	}); err != nil {
		return nil, err
	}

	out := make(map[int][]int, len(questWave))
	for quest, wave := range questWave {
		seen := map[int]struct{}{}
		var units []int
		for _, eid := range waveEnemies[wave] {
			uid, ok := enemyUnit[eid]
			if !ok {
				continue
			}
			if _, dup := seen[uid]; dup {
				continue
			}
			seen[uid] = struct{}{}
			units = append(units, uid)
		}
		sort.Ints(units)
		out[quest] = units
	}
	return out, nil
}
