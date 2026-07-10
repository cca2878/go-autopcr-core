package unit

import (
	"context"
	"fmt"
	"strings"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// maxMissingList 限制缺角色列表的展示条数，避免过长。
const maxMissingList = 40

// missingUnit 报告未获得的角色（图鉴缺口，只读）。母数据可获得角色集合 − 玩家持有集合。
type missingUnit struct{}

func (missingUnit) Meta() automation.Meta {
	return automation.Meta{
		Name:            "missing_unit",
		Title:           "查缺角色",
		Description:     "报告尚未获得的角色（区分限定/常驻，只读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (missingUnit) Params() []automation.Param { return nil }

func (missingUnit) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("查缺角色需要母数据，但未启用")
	}
	obtainable, err := md.Unit().Obtainables(ctx)
	if err != nil {
		return err
	}
	owned := gc.Data().OwnedUnitIDs()

	var limited, resident []string
	for _, u := range obtainable {
		if _, has := owned[u.UnitID]; has {
			continue
		}
		if u.IsLimited {
			limited = append(limited, u.Name)
		} else {
			resident = append(resident, u.Name)
		}
	}
	total := len(limited) + len(resident)
	if total == 0 {
		return automation.Skip("全图鉴玩家！没有缺少的角色")
	}
	rc.Logf("缺少 %d 个角色：限定 %d、常驻 %d", total, len(limited), len(resident))
	if len(limited) > 0 {
		rc.Logf("限定：%s", capList(limited))
	}
	if len(resident) > 0 {
		rc.Logf("常驻：%s", capList(resident))
	}
	return nil
}

// capList 连接名称列表，超过上限则截断并计数。
func capList(names []string) string {
	if len(names) > maxMissingList {
		return strings.Join(names[:maxMissingList], "、") + fmt.Sprintf(" 等 %d 个", len(names))
	}
	return strings.Join(names, "、")
}
