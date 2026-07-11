package daily

import (
	"context"
	"testing"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapiseason "github.com/cca2878/go-autopcr-core/internal/client/gameapi/seasonpass"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdseason "github.com/cca2878/go-autopcr-core/internal/client/masterdata/seasonpass"
)

// fakeSeasonAPI mock 女神祭域能力面，记录领取调用。
type fakeSeasonAPI struct {
	gapiseason.API
	receivable bool
	accepts    int
}

func (f *fakeSeasonAPI) Index(context.Context, int) (gapiseason.Index, error) {
	return gapiseason.Index{Level: 5, HasReceivableTask: f.receivable}, nil
}
func (f *fakeSeasonAPI) AcceptMissions(context.Context, int) (int, int, error) {
	f.accepts++
	return 3, 6, nil
}

// fakeSeasonMD mock 母数据女神祭域；fakeMDSeason 覆写 Reader.Seasonpass()。
type fakeSeasonMD struct {
	mdseason.API
	active []int
}

func (f *fakeSeasonMD) ActiveSeasonIDs(context.Context, time.Time) ([]int, error) {
	return f.active, nil
}

type fakeMDSeason struct {
	masterdata.Reader
	season mdseason.API
}

func (f *fakeMDSeason) Seasonpass() mdseason.API { return f.season }

func TestSeasonpassAccept(t *testing.T) {
	mk := func(active []int, receivable bool) (*moduletest.FakeClient, *fakeSeasonAPI) {
		api := &fakeSeasonAPI{receivable: receivable}
		fc := &moduletest.FakeClient{
			State:         &gamestate.PlayerState{},
			MD:            &fakeMDSeason{season: &fakeSeasonMD{active: active}},
			SeasonpassAPI: api,
		}
		return fc, api
	}

	// 有进行中女神祭且有可领任务 → 领取，成功。
	fc, api := mk([]int{1001}, true)
	if r := moduletest.RunOne(fc, seasonpassAccept{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有可领应成功: %+v", r)
	}
	if api.accepts != 1 {
		t.Fatalf("应领取一次: %d", api.accepts)
	}

	// 有进行中但无可领任务 → skip，不领取。
	fc2, api2 := mk([]int{1001}, false)
	if r := moduletest.RunOne(fc2, seasonpassAccept{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无可领应 skip: %+v", r)
	}
	if api2.accepts != 0 {
		t.Fatal("无可领不应领取")
	}

	// 无进行中女神祭 → skip。
	fc3, _ := mk(nil, true)
	if r := moduletest.RunOne(fc3, seasonpassAccept{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无进行中应 skip: %+v", r)
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: nil, SeasonpassAPI: &fakeSeasonAPI{}}
	if r := moduletest.RunOne(noMD, seasonpassAccept{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
