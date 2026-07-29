package daily

import (
	"context"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// charaFortune 抽取今日赛马（免费每日活动，获得宝石；对应 ref chara_fortune）。
type charaFortune struct{}

func (charaFortune) Meta() automation.Meta {
	return automation.Meta{
		Name:            "chara_fortune",
		Title:           "赛马",
		Description:     "赛马开放时段抽取今日赛马，获得宝石（每日一次，免费）",
		Category:        "收取",
		NeedsMasterdata: true,
	}
}

func (charaFortune) Params() []automation.Param { return nil }

func (charaFortune) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return automation.RequireMasterdata("判定赛马开放时段")
	}
	// 先查开放时段（母数据排程），再查今日是否已赛马（cf 状态），确有可抽再动作。
	open, err := md.Race().IsFortuneTime(ctx, time.Unix(gc.ServerTime(), 0))
	if err != nil {
		return err
	}
	if !open {
		return automation.Skip("今日无赛马")
	}
	cf := gc.Data().CharaFortune
	if cf == nil {
		return automation.Skip("今日已赛马")
	}
	got, err := gc.Race().DrawCharaFortune(ctx, cf.FortuneID, cf.UnitID)
	if err != nil {
		return err
	}
	rc.Logf("赛马第 %d 名，获得宝石 x%d", cf.Rank, got)
	return nil
}
