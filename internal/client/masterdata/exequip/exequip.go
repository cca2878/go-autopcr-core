// Package exequip 是母数据「EX 装备」域的只读查询（稀有度、名称、彩装副属性与炼成消耗等；
// 对应 ref ex_equipment_data / ex_equipment_sub_status(_group) / alces_cost / item_data）。
package exequip

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// exRarityNames 是 EX 装备稀有度名（对应 ref ex_rarity_name）。
var exRarityNames = map[int]string{1: "铜", 2: "银", 3: "金", 4: "粉", 5: "彩"}

// paramNameCh 是副属性(eParamType)中文名（对应 ref UnitAttribute.index2ch）。
var paramNameCh = map[int]string{
	1: "血量", 2: "物攻", 4: "魔攻", 3: "物防", 5: "魔防",
	6: "物爆", 7: "法爆", 10: "wave_hp_recovery", 11: "wave_energy_recovery",
	8: "闪避", 12: "物贯", 13: "法贯", 9: "吸血", 15: "hp_recovery_rate",
	14: "物爆提升", 16: "法爆提升", 17: "命中",
}

// paramIsPresent 是副属性是否为百分比值（对应 ref is_present，按属性直接映射）。
var paramIsPresent = map[int]bool{
	1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true,
	8: false, 9: false, 10: false, 11: false, 12: false, 13: false,
	14: false, 15: false, 16: false, 17: false,
}

// ItemKey 是库存物品 (类型, id) 键（对应 ref ItemType＝(eInventoryType, id)）。
type ItemKey struct {
	Type int
	ID   int
}

// RainbowEnhancePt 是彩装究极炼成 PT 的库存键（对应 ref ex_rainbow_enhance_pt＝(Item, 26202)）。
var RainbowEnhancePt = ItemKey{Type: 2, ID: 26202}

// ParamNameCh 返回某副属性(eParamType)的中文名（未知→"未知属性N"）。
func ParamNameCh(status int) string {
	if n, ok := paramNameCh[status]; ok {
		return n
	}
	return fmt.Sprintf("未知属性%d", status)
}

// statusByNameCh 是 paramNameCh 的反查（中文→status）。
var statusByNameCh = func() map[string]int {
	m := make(map[string]int, len(paramNameCh))
	for st, n := range paramNameCh {
		m[n] = st
	}
	return m
}()

// StatusByNameCh 把副属性中文名转为 status（"任意"/""→0；未知→0）。
func StatusByNameCh(name string) int {
	if name == "" || name == "任意" {
		return 0
	}
	return statusByNameCh[name]
}

// SubStatusEntry 是格式化用的一条副属性（status + step）。
type SubStatusEntry struct {
	Status int
	Step   int
}

// API 是 EX 装备域查询契约（随功能在本包内累加）。
type API interface {
	// RarityByID 返回全部 EX 装备的 ex_equipment_id→rarity 映射（供按稀有度计数）。
	RarityByID(ctx context.Context) (map[int]int, error)
	// AlcesCost 返回究极炼成的单次基础材料消耗（(类型,id)→count；对应 ref alces_cost）。
	AlcesCost(ctx context.Context) (map[ItemKey]int, error)
	// LoadSnapshot 一次性加载彩装炼成所需的全部母数据（名称/稀有度/副属性表/物品名），
	// 返回内存快照以避免逐次炼成打 DB。
	LoadSnapshot(ctx context.Context) (*Snapshot, error)
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

func (a *Impl) AlcesCost(ctx context.Context) (map[ItemKey]int, error) {
	out := make(map[ItemKey]int)
	err := a.scan(ctx, "SELECT type, item_id, count FROM alces_cost", func(rows scanner) error {
		var typ, id, count int
		if err := rows.Scan(&typ, &id, &count); err != nil {
			return err
		}
		out[ItemKey{Type: typ, ID: id}] = count
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Snapshot 是彩装炼成所需母数据的内存快照（一次加载、多次使用）。
type Snapshot struct {
	rarity   map[int]int            // ex_equipment_id → rarity
	rawName  map[int]string         // ex_equipment_id → 装备名（无稀有度前缀）
	group    map[int]int            // ex_equipment_id → group_id
	values   map[int]map[int][5]int // group_id → status → [step1..5] 属性值
	itemName map[int]string         // item_id → 名称（材料展示用）
}

// LoadSnapshot 一次性加载彩装炼成所需的全部母数据。
func (a *Impl) LoadSnapshot(ctx context.Context) (*Snapshot, error) {
	s := &Snapshot{
		rarity:   map[int]int{},
		rawName:  map[int]string{},
		group:    map[int]int{},
		values:   map[int]map[int][5]int{},
		itemName: map[int]string{},
	}

	if err := a.scan(ctx, "SELECT ex_equipment_id, rarity, name FROM ex_equipment_data", func(rows scanner) error {
		var id, rarity int
		var name string
		if err := rows.Scan(&id, &rarity, &name); err != nil {
			return err
		}
		s.rarity[id] = rarity
		s.rawName[id] = name
		return nil
	}); err != nil {
		return nil, err
	}

	if err := a.scan(ctx, "SELECT ex_equipment_id, group_id FROM ex_equipment_sub_status_group", func(rows scanner) error {
		var id, gid int
		if err := rows.Scan(&id, &gid); err != nil {
			return err
		}
		s.group[id] = gid
		return nil
	}); err != nil {
		return nil, err
	}

	if err := a.scan(ctx, "SELECT group_id, status, value_1, value_2, value_3, value_4, value_5 FROM ex_equipment_sub_status", func(rows scanner) error {
		var gid, status int
		var v [5]int
		if err := rows.Scan(&gid, &status, &v[0], &v[1], &v[2], &v[3], &v[4]); err != nil {
			return err
		}
		if s.values[gid] == nil {
			s.values[gid] = map[int][5]int{}
		}
		s.values[gid][status] = v
		return nil
	}); err != nil {
		return nil, err
	}

	if err := a.scan(ctx, "SELECT item_id, item_name FROM item_data", func(rows scanner) error {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		s.itemName[id] = name
		return nil
	}); err != nil {
		return nil, err
	}

	return s, nil
}

// scanner 抽象 *sql.Rows 的 Scan（便于 scan 复用）。
type scanner interface{ Scan(dest ...any) error }

// scan 执行 query 并对每行调用 fn。
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

// NewSnapshot 用给定映射在内存中装配快照（rarity/名称按 ex_equipment_id，group 按 ex_equipment_id→group_id，
// values 按 group_id→status→[step1..5]，itemNames 按 item_id）。主要供测试与在内存构造使用。
func NewSnapshot(rarity, group map[int]int, names, itemNames map[int]string, values map[int]map[int][5]int) *Snapshot {
	return &Snapshot{rarity: rarity, rawName: names, group: group, values: values, itemName: itemNames}
}

// Rarity 返回某 EX 装备稀有度（未知→0）。
func (s *Snapshot) Rarity(exEquipmentID int) int { return s.rarity[exEquipmentID] }

// ExEquipName 返回 "{稀有度名}-{装备名}"（对应 ref get_ex_equip_name，略 rank）。
func (s *Snapshot) ExEquipName(exEquipmentID int) string {
	name, ok := s.rawName[exEquipmentID]
	if !ok {
		return fmt.Sprintf("未知ex装备(%d)", exEquipmentID)
	}
	return exRarityNames[s.rarity[exEquipmentID]] + "-" + name
}

// ItemName 返回某物品 id 的名称（未知→"未知物品(id)"）。
func (s *Snapshot) ItemName(itemID int) string {
	if n, ok := s.itemName[itemID]; ok {
		return n
	}
	return fmt.Sprintf("未知物品(%d)", itemID)
}

// SubStatusCandidates 返回本快照中出现过的全部副属性(status)，升序去重——即彩装副属性的候选集
// （对应 ref ex_equip_sub_status_candidate：取 ex_equipment_sub_status 全表 distinct status）。
// 不含 0：ref 里的 0＝「任意」是配置层语义，由模块自行添加。
//
// 从已加载的快照里收集而不另发查询，故与炼成主流程共用同一次 LoadSnapshot。
func (s *Snapshot) SubStatusCandidates() []int {
	seen := map[int]bool{}
	for _, byStatus := range s.values {
		for st := range byStatus {
			seen[st] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// StatusSupported 报告某 EX 装备是否支持某副属性（对应 ref status not in sub_status_data 的反）。
func (s *Snapshot) StatusSupported(exEquipmentID, status int) bool {
	vals, ok := s.values[s.group[exEquipmentID]]
	if !ok {
		return false
	}
	_, ok = vals[status]
	return ok
}

// SubStatusStr 把一组副属性格式化为 "血量x123/物攻x45.00%"（对应 ref get_ex_equip_sub_status_str）：
// 按属性求 step 值之和，百分比属性除 100 并以两位小数百分号展示。空→"空"。
func (s *Snapshot) SubStatusStr(exEquipmentID int, subs []SubStatusEntry) string {
	vals := s.values[s.group[exEquipmentID]]
	sum := map[int]int{}
	for _, e := range subs {
		if e.Step >= 1 && e.Step <= 5 {
			sum[e.Status] += vals[e.Status][e.Step-1]
		}
	}
	if len(sum) == 0 {
		return "空"
	}
	statuses := slices.Sorted(maps.Keys(sum))
	parts := make([]string, 0, len(statuses))
	for _, st := range statuses {
		name := ParamNameCh(st)
		if paramIsPresent[st] {
			parts = append(parts, fmt.Sprintf("%sx%.2f%%", name, float64(sum[st])/100))
		} else {
			parts = append(parts, fmt.Sprintf("%sx%d", name, sum[st]))
		}
	}
	return strings.Join(parts, "/")
}
