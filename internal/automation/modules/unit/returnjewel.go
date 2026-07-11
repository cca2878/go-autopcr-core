package unit

import (
	"context"
	"math"
	"sort"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// 返钻公式常量（复刻 ref return_jewel）：等级同步只统计练度最高的 20 个之外的角色。
const (
	syncTopCount      = 20  // 等级同步保留的角色数（前 20 不计入返钻）
	maxPromotionLevel = 31  // 满品级基准（ref 用于「最大 box 返钻」与满品级计数）
	maxUnitLevel      = 305 // 满等级基准
	returnJewelBase   = 1500
)

// returnJewel 计算等级同步（转生/重置）的返钻数量（纯读，仅用登录折叠的持有角色练度）。
type returnJewel struct{}

func (returnJewel) Meta() automation.Meta {
	return automation.Meta{
		Name:        "return_jewel",
		Title:       "返钻计算",
		Description: "计算等级同步可返还的钻石数量（只读，不消耗资源）",
		Category:    "查询",
	}
}

func (returnJewel) Params() []automation.Param { return nil }

func (returnJewel) Run(_ context.Context, gc client.GameClient, rc *automation.RunContext) error {
	units := gc.Data().Units
	n := len(units)
	if n == 0 {
		return automation.Skip("无角色数据")
	}

	promo := make([]int, n)
	lvl := make([]int, n)
	countMaxPromotion, countMaxLevel := 0, 0
	for i, u := range units {
		promo[i] = u.PromotionLevel
		lvl[i] = u.Level
		if u.PromotionLevel == maxPromotionLevel {
			countMaxPromotion++
		}
		if u.Level == maxUnitLevel {
			countMaxLevel++
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(promo)))
	sort.Sort(sort.Reverse(sort.IntSlice(lvl)))

	// 前 syncTopCount 个练度最高的角色不返钻，其余各按 (值-1) 累加。
	value1 := sumTail(promo, syncTopCount)
	value2 := sumTail(lvl, syncTopCount)
	returnJewelCount := returnJewelBase + (float64(value1)+float64(value2)/10)/2
	returnJewel10 := math.Ceil(returnJewelCount/10) * 10
	maxReturn := math.Ceil((returnJewelBase+float64(n-syncTopCount)*(float64(maxPromotionLevel-1)+float64(maxUnitLevel-1)/10)/2)/10) * 10

	rc.Logf("当前角色数：%d", n)
	rc.Logf("满品级角色数：%d ｜ 满等级角色数：%d", countMaxPromotion, countMaxLevel)
	rc.Logf("返钻数量：%.1f（向上取整实得：%.0f）", returnJewelCount, returnJewel10)
	rc.Logf("当前 box 最多返钻：%.0f", maxReturn)
	return nil
}

// sumTail 返回把 vals（已降序）跳过前 skip 个后，其余各减 1 的和；不足 skip 个则为 0。
func sumTail(vals []int, skip int) int {
	if len(vals) <= skip {
		return 0
	}
	sum := 0
	for _, v := range vals[skip:] {
		sum += v - 1
	}
	return sum
}
