package story

import (
	"context"
	"testing"
	"time"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	mdstory "github.com/cca2878/go-autopcr/internal/client/masterdata/story"
)

// fakeStoryData mock 母数据剧情域；fakeStoryReader 覆写访问器 Story()。
type fakeStoryData struct {
	mdstory.API
	birthday func(ctx context.Context) ([]mdstory.Story, error)
	main     func(ctx context.Context) ([]mdstory.Story, error)
	unit     func(ctx context.Context) ([]mdstory.UnitStory, error)
}

func (f *fakeStoryData) BirthdayStories(ctx context.Context) ([]mdstory.Story, error) {
	return f.birthday(ctx)
}
func (f *fakeStoryData) MainStories(ctx context.Context) ([]mdstory.Story, error) { return f.main(ctx) }
func (f *fakeStoryData) UnitStories(ctx context.Context) ([]mdstory.UnitStory, error) {
	return f.unit(ctx)
}

type fakeStoryReader struct {
	masterdata.Reader
	story mdstory.API
}

func (f *fakeStoryReader) Story() mdstory.API { return f.story }

func mkStoryClient(state *gamestate.PlayerState, data *fakeStoryData) *moduletest.FakeClient {
	return &moduletest.FakeClient{
		State:         state,
		ServerTimeVal: time.Now().Unix(),
		MD:            &fakeStoryReader{story: data},
	}
}

func TestBirthdayStoryReport(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)
	stories := []mdstory.Story{
		{StoryID: 4000, PreStoryID: 0, StartTime: past, Title: "已读篇"},
		{StoryID: 4001, PreStoryID: 0, StartTime: past, Title: "可读篇一"},
		{StoryID: 4002, PreStoryID: 4001, StartTime: past, Title: "可读篇二"},
		{StoryID: 4003, PreStoryID: 0, StartTime: future, Title: "未解锁篇"},
	}
	fd := &fakeStoryData{birthday: func(context.Context) ([]mdstory.Story, error) { return stories, nil }}

	if r := moduletest.RunOne(mkStoryClient(&gamestate.PlayerState{ReadStoryIDs: []int{4000}}, fd), birthdayStoryReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("应报告可读篇: %+v", r)
	}
	if r := moduletest.RunOne(mkStoryClient(&gamestate.PlayerState{ReadStoryIDs: []int{4000, 4001, 4002}}, fd), birthdayStoryReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无可读应 skip: %+v", r)
	}
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: nil}
	if r := moduletest.RunOne(noMD, birthdayStoryReport{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}

func TestMainStoryReport(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	stories := []mdstory.Story{
		{StoryID: 2000, PreStoryID: 0, UnlockQuestID: 0, StartTime: past, Title: "序章"},
		{StoryID: 2001, PreStoryID: 0, UnlockQuestID: 0, StartTime: past, Title: "第1话"},
		{StoryID: 2002, PreStoryID: 2001, UnlockQuestID: 900, StartTime: past, Title: "第2话"},
		{StoryID: 2003, PreStoryID: 2002, UnlockQuestID: 999, StartTime: past, Title: "第3话"},
	}
	fd := &fakeStoryData{main: func(context.Context) ([]mdstory.Story, error) { return stories, nil }}

	ok := &gamestate.PlayerState{ReadStoryIDs: []int{2000}, ClearedQuests: map[int]struct{}{900: {}}}
	if r := moduletest.RunOne(mkStoryClient(ok, fd), mainStoryReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("应报告可读主线: %+v", r)
	}
	done := &gamestate.PlayerState{ReadStoryIDs: []int{2000, 2001, 2002}, ClearedQuests: map[int]struct{}{900: {}}}
	if r := moduletest.RunOne(mkStoryClient(done, fd), mainStoryReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("门禁未通关应 skip: %+v", r)
	}
	byway := &gamestate.PlayerState{ReadStoryIDs: []int{2000, 2001}, ClearedBywayQuests: map[int]struct{}{900: {}}}
	if r := moduletest.RunOne(mkStoryClient(byway, fd), mainStoryReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("支线通关集合应满足门禁: %+v", r)
	}
}

func TestUnitStoryReport(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	far := time.Now().Add(100 * 365 * 24 * time.Hour)
	stories := []mdstory.UnitStory{
		{StoryID: 1001001, StoryGroupID: 1001, PreStoryID: 0, LoveLevel: 0, ReadProcessFlag: true, ForceUnlockTime: far, ForceUnlockTime2: far, StartTime: past, EndTime: far, Title: "1001-1"},
		{StoryID: 1001002, StoryGroupID: 1001, PreStoryID: 1001001, LoveLevel: 2, ReadProcessFlag: true, ForceUnlockTime: far, ForceUnlockTime2: far, StartTime: past, EndTime: far, Title: "1001-2"},
		{StoryID: 1001003, StoryGroupID: 1001, PreStoryID: 1001002, LoveLevel: 6, ReadProcessFlag: true, ForceUnlockTime: far, ForceUnlockTime2: far, StartTime: past, EndTime: far, Title: "1001-3"},
		{StoryID: 1002001, StoryGroupID: 1002, PreStoryID: 0, LoveLevel: 0, ReadProcessFlag: true, ForceUnlockTime: far, ForceUnlockTime2: far, StartTime: past, EndTime: far, Title: "1002-1"},
		{StoryID: 1255001, StoryGroupID: mdstory.MagicalGirlGroupID, PreStoryID: 0, LoveLevel: 0, ReadProcessFlag: true, ForceUnlockTime: far, ForceUnlockTime2: far, StartTime: past, EndTime: far, Title: "魔姬"},
	}
	fd := &fakeStoryData{unit: func(context.Context) ([]mdstory.UnitStory, error) { return stories, nil }}

	if r := moduletest.RunOne(mkStoryClient(&gamestate.PlayerState{UnitLove: map[int]int{1001: 5}}, fd), unitStoryReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("持有角色应报告可读: %+v", r)
	}
	if r := moduletest.RunOne(mkStoryClient(&gamestate.PlayerState{UnitLove: map[int]int{1001: 6}}, fd), unitStoryReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("好感 6 应含第3篇: %+v", r)
	}
	if r := moduletest.RunOne(mkStoryClient(&gamestate.PlayerState{UnitLove: nil}, fd), unitStoryReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未持有角色应 skip: %+v", r)
	}
	allRead := &gamestate.PlayerState{ReadStoryIDs: []int{1001001, 1001002, 1001003}, UnitLove: map[int]int{1001: 6}}
	if r := moduletest.RunOne(mkStoryClient(allRead, fd), unitStoryReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("全部已读应 skip: %+v", r)
	}
}
