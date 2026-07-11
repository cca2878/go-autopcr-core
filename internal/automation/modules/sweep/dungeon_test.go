package sweep

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapidungeon "github.com/cca2878/go-autopcr-core/internal/client/gameapi/dungeon"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mddungeon "github.com/cca2878/go-autopcr-core/internal/client/masterdata/dungeon"
)

// fakeDungeonAPI mock 地下城域能力面。
type fakeDungeonAPI struct {
	gapidungeon.API
	info gapidungeon.Info
}

func (f *fakeDungeonAPI) Info(context.Context) (gapidungeon.Info, error) { return f.info, nil }

// fakeDungeonMD mock 母数据地下城域；fakeReader 覆写 Reader.Dungeon()。
type fakeDungeonMD struct {
	mddungeon.API
}

func (fakeDungeonMD) AreaName(_ context.Context, id int) (string, error) { return "测试区域", nil }

type fakeReader struct {
	masterdata.Reader
}

func (fakeReader) Dungeon() mddungeon.API { return fakeDungeonMD{} }

func TestExploreManaReport(t *testing.T) {
	fc := &moduletest.FakeClient{State: &gamestate.PlayerState{TrainingManaDone: 0, TrainingManaMax: 1}}
	if r := moduletest.RunOne(fc, exploreManaReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有剩余应成功: %+v", r)
	}
	done := &moduletest.FakeClient{State: &gamestate.PlayerState{TrainingManaDone: 1, TrainingManaMax: 1}}
	if r := moduletest.RunOne(done, exploreManaReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("已扫荡完应 skip: %+v", r)
	}
}

func TestDungeonReport(t *testing.T) {
	md := fakeReader{}

	// 有剩余挑战 → 成功。
	fc := &moduletest.FakeClient{
		State:      &gamestate.PlayerState{},
		MD:         md,
		DungeonAPI: &fakeDungeonAPI{info: gapidungeon.Info{EnterAreaID: 101, RestChallenge: 1, MaxChallenge: 1}},
	}
	if r := moduletest.RunOne(fc, dungeonReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有可挑战应成功: %+v", r)
	}

	// 无剩余 → skip。
	fc2 := &moduletest.FakeClient{
		State:      &gamestate.PlayerState{},
		MD:         md,
		DungeonAPI: &fakeDungeonAPI{info: gapidungeon.Info{EnterAreaID: 0, RestChallenge: 0, MaxChallenge: 1}},
	}
	if r := moduletest.RunOne(fc2, dungeonReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无可挑战应 skip: %+v", r)
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: nil, DungeonAPI: &fakeDungeonAPI{}}
	if r := moduletest.RunOne(noMD, dungeonReport{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
