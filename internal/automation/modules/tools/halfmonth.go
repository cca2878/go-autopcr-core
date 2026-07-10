package tools

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// halfMonth 汇总当前与未来的活动日程（半月刊；对应 ref half_schedule，只读、纯母数据）。
//
// 收录的排程表见 masterdata/schedule（当前为公会战/女神祭/活动/露娜塔/庆典的高信号子集，其余
// 同类表按同法追加）。只列出「尚未结束」的排程，按开始时间排序，未来项单列。
type halfMonth struct{}

func (halfMonth) Meta() automation.Meta {
	return automation.Meta{
		Name:            "half_month",
		Title:           "半月刊",
		Description:     "汇总当前与未来的活动日程（公会战/女神祭/活动/露娜塔/庆典等，只读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (halfMonth) Params() []automation.Param { return nil }

func (halfMonth) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("半月刊需要母数据，但未启用")
	}
	entries, err := md.Schedule().Schedules(ctx)
	if err != nil {
		return err
	}
	now := time.Unix(gc.ServerTime(), 0)

	// 只保留尚未结束的排程，按开始时间升序。
	upcoming := entries[:0]
	for _, e := range entries {
		if e.End.After(now) {
			upcoming = append(upcoming, e)
		}
	}
	if len(upcoming) == 0 {
		return automation.Skip("无当前或未来的活动日程")
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].Start.Before(upcoming[j].Start) })

	const dateFmt = "2006-01-02"
	future := false
	seen := make(map[string]struct{}) // 同一 (时段,系统,说明) 去重：庆典等同窗口多行合并为一条
	for _, e := range upcoming {
		if !future && e.Start.After(now) {
			future = true
			rc.Logf("──── 未来日程 ────")
		}
		label := e.Label
		if e.Detail != "" {
			label += "：" + e.Detail
		}
		line := fmt.Sprintf("%s ~ %s  %s", e.Start.Format(dateFmt), e.End.Format(dateFmt), label)
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		rc.Logf("%s", line)
	}
	return nil
}
