package room

import (
	"context"
	"fmt"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// maxLoveUpReportNames 是报告里最多列出的角色名数量（其余以计数概括，避免刷屏）。
const maxLoveUpReportNames = 30

// loveUpReport 报告'亲密度未满、喂蛋糕可提升'的角色（只读，不实际喂食）。
//
// 对应参考项目 love_up（喂蛋糕）：原模块会消耗蛋糕提升亲密度；此处为验证 masterdata 设计一律
// 降级为只读报告，只统计差多少亲密度、可升至几级，不发送 give_gift 写请求。
type loveUpReport struct{}

func (loveUpReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "love_up",
		Title:           "喂蛋糕（可提升报告）",
		Description:     "报告亲密度未满、可喂蛋糕提升的角色（只读，不实际喂食/消耗蛋糕）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (loveUpReport) Params() []automation.Param { return nil }

func (loveUpReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return automation.RequireMasterdata("生成喂蛋糕报告")
	}
	data := gc.Data()

	// 按星级缓存亲密度上限，避免逐角色重复查询。
	type loveCap struct{ level, total int }
	capByRarity := make(map[int]loveCap)
	maxTotalLove := func(rarity int) (loveCap, error) {
		if c, ok := capByRarity[rarity]; ok {
			return c, nil
		}
		lvl, total, err := md.Unit().MaxTotalLove(ctx, rarity)
		if err != nil {
			return loveCap{}, err
		}
		c := loveCap{level: lvl, total: total}
		capByRarity[rarity] = c
		return c, nil
	}

	var names []string
	notMaxed := 0
	for _, u := range data.Units {
		charaID := u.ID / 100
		cur, ok := data.CharaLove[charaID]
		if !ok {
			continue // 无亲密度记录（未建立羁绊），跳过
		}
		lc, err := maxTotalLove(u.Rarity)
		if err != nil {
			return err
		}
		if cur >= lc.total {
			continue // 已满
		}
		notMaxed++
		if len(names) < maxLoveUpReportNames {
			name, err := md.Unit().Name(ctx, u.ID)
			if err != nil {
				return err
			}
			names = append(names, fmt.Sprintf("%s（差 %d，可升至 %d 级）", name, lc.total-cur, lc.level))
		}
	}

	if notMaxed == 0 {
		return automation.Skip("所有角色亲密度均已满级")
	}
	rc.Logf("亲密度未满角色：%d", notMaxed)
	rc.Logf("%s", strings.Join(names, "\n"))
	if notMaxed > len(names) {
		rc.Logf("……另有 %d 个未列出", notMaxed-len(names))
	}
	return nil
}
