package daily

import (
	"context"
	"testing"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapirace "github.com/cca2878/go-autopcr-core/internal/client/gameapi/race"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdrace "github.com/cca2878/go-autopcr-core/internal/client/masterdata/race"
)

// fakeRace mock 赛马域能力面（抽取），记录调用参数。
type fakeRace struct {
	gapirace.API
	got                     int
	fortuneID, unitID, hits int
}

func (f *fakeRace) DrawCharaFortune(_ context.Context, fortuneID, unitID int) (int, error) {
	f.fortuneID, f.unitID, f.hits = fortuneID, unitID, f.hits+1
	return f.got, nil
}

// fakeRaceMD mock 母数据赛马排程；fakeMDRace 覆写 Reader.Race()。
type fakeRaceMD struct {
	mdrace.API
	open bool
}

func (f *fakeRaceMD) IsFortuneTime(context.Context, time.Time) (bool, error) { return f.open, nil }

type fakeMDRace struct {
	masterdata.Reader
	race mdrace.API
}

func (f *fakeMDRace) Race() mdrace.API { return f.race }

func TestCharaFortune(t *testing.T) {
	cf := &gamestate.CharaFortune{FortuneID: 7, UnitID: 100101, Rank: 3}

	// 开放且今日未赛马 → 抽取成功，参数正确。
	fr := &fakeRace{got: 50}
	fc := &moduletest.FakeClient{
		State:   &gamestate.PlayerState{CharaFortune: cf},
		MD:      &fakeMDRace{race: &fakeRaceMD{open: true}},
		RaceAPI: fr,
	}
	if r := moduletest.RunOne(fc, charaFortune{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("可赛马应成功: %+v", r)
	}
	if fr.hits != 1 || fr.fortuneID != 7 || fr.unitID != 100101 {
		t.Fatalf("抽取参数错误: %+v", fr)
	}

	// 非开放时段 → skip，不抽取。
	fr2 := &fakeRace{}
	fc2 := &moduletest.FakeClient{
		State:   &gamestate.PlayerState{CharaFortune: cf},
		MD:      &fakeMDRace{race: &fakeRaceMD{open: false}},
		RaceAPI: fr2,
	}
	if r := moduletest.RunOne(fc2, charaFortune{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("非开放应 skip: %+v", r)
	}
	if fr2.hits != 0 {
		t.Fatal("非开放不应抽取")
	}

	// 开放但今日已赛马（cf==nil）→ skip。
	fr3 := &fakeRace{}
	fc3 := &moduletest.FakeClient{
		State:   &gamestate.PlayerState{CharaFortune: nil},
		MD:      &fakeMDRace{race: &fakeRaceMD{open: true}},
		RaceAPI: fr3,
	}
	if r := moduletest.RunOne(fc3, charaFortune{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("已赛马应 skip: %+v", r)
	}
	if fr3.hits != 0 {
		t.Fatal("已赛马不应抽取")
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{CharaFortune: cf}, MD: nil}
	if r := moduletest.RunOne(noMD, charaFortune{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
