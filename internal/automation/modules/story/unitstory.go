package story

import (
	"context"
	"fmt"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	mdstory "github.com/cca2878/go-autopcr-core/internal/client/masterdata/story"
)

// unitStoryReport 报告【可阅读但未读】的角色好感剧情（只读，不实际阅读）。最典型的 masterdata
// 交叉：门禁取决于母数据里每篇的好感等级要求与玩家状态里该角色当前好感等级的比较。
type unitStoryReport struct{}

func (unitStoryReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "unit_story",
		Title:           "角色好感剧情（可读报告）",
		Description:     "报告当前可阅读但未读的角色好感剧情（只读，不实际阅读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (unitStoryReport) Params() []automation.Param { return nil }

func (unitStoryReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("角色好感剧情报告需要母数据，但未启用")
	}
	stories, err := md.Story().UnitStories(ctx)
	if err != nil {
		return err
	}
	data := gc.Data()
	read := data.ReadStorySet()
	love := data.UnitLove
	now := time.Unix(gc.ServerTime(), 0)

	var readable []string
	for _, s := range stories {
		if s.StoryGroupID == mdstory.MagicalGirlGroupID || !s.ReadProcessFlag {
			continue // 忽略魔姬剧情 / 非可读剧情
		}
		if _, done := read[s.StoryID]; done {
			continue // 已读
		}
		if !preConditionMet(read, s.PreStoryID, now, s.ForceUnlockTime) ||
			!preConditionMet(read, s.PreStoryID2, now, s.ForceUnlockTime2) {
			continue // 前置未读且未到强制解锁时间
		}
		lv, owned := love[s.StoryGroupID]
		if !owned || lv < s.LoveLevel {
			continue // 未持有该角色，或好感不足
		}
		if now.Before(s.StartTime) || now.After(s.EndTime) {
			continue // 不在上架时间窗内
		}
		readable = append(readable, s.Title)
		read[s.StoryID] = struct{}{} // 视为已读，链式解锁同角色续篇
	}
	if len(readable) == 0 {
		return automation.Skip("没有可阅读的角色好感剧情")
	}
	rc.Logf("有 %d 篇可阅读的角色好感剧情：%s", len(readable), formatReadable(readable))
	return nil
}

// preConditionMet 判定单个前置条件：前置剧情已读，或已到强制解锁时间。
func preConditionMet(read map[int]struct{}, preStoryID int, now, forceUnlock time.Time) bool {
	if _, ok := read[preStoryID]; ok {
		return true
	}
	return !now.Before(forceUnlock)
}
