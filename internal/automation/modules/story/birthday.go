package story

import (
	"context"
	"fmt"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// birthdayStoryReport 报告【可阅读但未读】的生日剧情（只读，不实际阅读）。
type birthdayStoryReport struct{}

func (birthdayStoryReport) Meta() automation.Meta {
	return automation.Meta{
		Name:            "birthday_story",
		Title:           "生日剧情（可读报告）",
		Description:     "报告当前可阅读但未读的生日剧情（只读，不实际阅读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (birthdayStoryReport) Params() []automation.Param { return nil }

func (birthdayStoryReport) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("生日剧情报告需要母数据，但未启用")
	}
	stories, err := md.Story().BirthdayStories(ctx)
	if err != nil {
		return err
	}
	read := gc.Data().ReadStorySet()
	now := time.Unix(gc.ServerTime(), 0)

	var readable []string
	for _, s := range stories {
		if now.Before(s.StartTime) {
			continue // 尚未解锁
		}
		if _, done := read[s.StoryID]; done {
			continue // 已读
		}
		if _, ok := read[s.PreStoryID]; !ok {
			continue // 前置未读
		}
		readable = append(readable, s.Title)
		read[s.StoryID] = struct{}{} // 视为已读，使后续篇章前置判定成立（复刻 ref 链式解锁）
	}
	if len(readable) == 0 {
		return automation.Skip("没有可阅读的生日剧情")
	}
	rc.Logf("有 %d 篇可阅读的生日剧情：%s", len(readable), formatReadable(readable))
	return nil
}
