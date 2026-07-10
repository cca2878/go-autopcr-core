package story

import (
	"context"
	"fmt"
	"time"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/client"
)

// mainStoryReport 报告【可阅读但未读】的主线/支线剧情（只读，不实际阅读）。比生日剧情多一道
// 任务通关门禁（unlock_quest_id），需 home/index 折叠的通关状态。
type mainStoryReport struct{}

func (mainStoryReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "main_story",
		Title:           "主线剧情（可读报告）",
		Description:     "报告当前可阅读但未读的主线/支线剧情（只读，不实际阅读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (mainStoryReport) Params() []automation.Param { return nil }

func (mainStoryReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("主线剧情报告需要母数据，但未启用")
	}
	stories, err := md.Story().MainStories(ctx)
	if err != nil {
		return err
	}
	data := gc.Data()
	read := data.ReadStorySet()
	now := time.Unix(gc.ServerTime(), 0)

	var readable []string
	locked := 0
	for _, s := range stories {
		if now.Before(s.StartTime) {
			continue // 尚未解锁（时间）
		}
		if _, done := read[s.StoryID]; done {
			continue // 已读
		}
		if _, ok := read[s.PreStoryID]; !ok {
			continue // 前置未读
		}
		if !data.IsQuestUnlocked(s.UnlockQuestID) {
			locked++ // 关联任务未通关
			continue
		}
		readable = append(readable, s.Title)
		read[s.StoryID] = struct{}{} // 视为已读，链式解锁续篇
	}
	if len(readable) == 0 {
		return automation.Skip("没有可阅读的主线/支线剧情")
	}
	rc.Logf("有 %d 篇可阅读的主线/支线剧情：%s", len(readable), formatReadable(readable))
	if locked > 0 {
		rc.Logf("另有 %d 篇因关联任务未通关暂不可读", locked)
	}
	return nil
}
