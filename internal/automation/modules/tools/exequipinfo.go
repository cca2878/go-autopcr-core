package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// exRarityNames 是 EX 装备稀有度名（对应 ref db.ex_rarity_name）。
var exRarityNames = map[int]string{1: "铜", 2: "银", 3: "金", 4: "粉", 5: "彩"}

// exEquipInfo 报告玩家 EX 装备按稀有度的持有数量（只读；对应 ref ex_equip_info，简化为计数）。
type exEquipInfo struct{}

func (exEquipInfo) Meta() automation.Meta {
	return automation.Meta{
		Name:            "ex_equip_info",
		Title:           "查ex装备",
		Description:     "报告 EX 装备按稀有度的持有数量（只读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (exEquipInfo) Params() []automation.Param { return nil }

func (exEquipInfo) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("查ex装备需要母数据解析稀有度，但未启用")
	}
	ids := gc.Data().ExEquipIDs
	if len(ids) == 0 {
		return automation.Skip("无 EX 装备")
	}
	rarityByID, err := md.Exequip().RarityByID(ctx)
	if err != nil {
		return err
	}

	countByRarity := make(map[int]int)
	for _, id := range ids {
		countByRarity[rarityByID[id]]++ // 未知 id → rarity 0，归入"其他"
	}

	rarities := make([]int, 0, len(countByRarity))
	for r := range countByRarity {
		rarities = append(rarities, r)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(rarities))) // 高稀有度在前

	var parts []string
	for _, r := range rarities {
		name := exRarityNames[r]
		if name == "" {
			name = "其他"
		}
		parts = append(parts, fmt.Sprintf("%s x%d", name, countByRarity[r]))
	}
	rc.Logf("EX 装备共 %d 件：%s", len(ids), strings.Join(parts, " ｜ "))
	return nil
}
