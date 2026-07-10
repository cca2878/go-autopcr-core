package tools

import (
	"context"
	"testing"
	"time"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	mdschedule "github.com/cca2878/go-autopcr/internal/client/masterdata/schedule"
)

// fakeScheduleMD mock 母数据日程域；fakeSchedReader 覆写 Reader.Schedule()。
type fakeScheduleMD struct {
	mdschedule.API
	entries []mdschedule.Entry
}

func (f *fakeScheduleMD) Schedules(context.Context) ([]mdschedule.Entry, error) {
	return f.entries, nil
}

type fakeSchedReader struct {
	masterdata.Reader
	schedule mdschedule.API
}

func (f *fakeSchedReader) Schedule() mdschedule.API { return f.schedule }

func TestHalfMonth(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	day := 24 * time.Hour
	entries := []mdschedule.Entry{
		{Start: now.Add(-10 * day), End: now.Add(-5 * day), Label: "已结束活动"},          // 应过滤
		{Start: now.Add(-2 * day), End: now.Add(3 * day), Label: "公会战"},              // 进行中
		{Start: now.Add(5 * day), End: now.Add(9 * day), Label: "女神祭", Detail: "夏日"}, // 未来
	}
	fc := &moduletest.FakeClient{
		State:         &gamestate.PlayerState{},
		ServerTimeVal: now.Unix(),
		MD:            &fakeSchedReader{schedule: &fakeScheduleMD{entries: entries}},
	}
	if r := moduletest.RunOne(fc, halfMonth{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有日程应成功: %+v", r)
	}

	// 全部已结束 → skip。
	past := []mdschedule.Entry{{Start: now.Add(-10 * day), End: now.Add(-5 * day), Label: "旧活动"}}
	fcPast := &moduletest.FakeClient{
		State:         &gamestate.PlayerState{},
		ServerTimeVal: now.Unix(),
		MD:            &fakeSchedReader{schedule: &fakeScheduleMD{entries: past}},
	}
	if r := moduletest.RunOne(fcPast, halfMonth{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("全过期应 skip: %+v", r)
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, ServerTimeVal: now.Unix(), MD: nil}
	if r := moduletest.RunOne(noMD, halfMonth{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
