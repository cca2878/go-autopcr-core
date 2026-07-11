package daily

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapimirage "github.com/cca2878/go-autopcr-core/internal/client/gameapi/mirage"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdmirage "github.com/cca2878/go-autopcr-core/internal/client/masterdata/mirage"
)

// fakeMirageAPI mock 追忆战域能力面。
type fakeMirageAPI struct {
	gapimirage.API
	fullTime int64
	received int
	receives int
}

func (f *fakeMirageAPI) RewardFullTime(context.Context) (int64, error) { return f.fullTime, nil }
func (f *fakeMirageAPI) ReceiveReward(context.Context, int) (int, error) {
	f.receives++
	return f.received, nil
}

// fakeMirageMD mock 母数据追忆战域；fakeMDMirage 覆写 Reader.Mirage()。
type fakeMirageMD struct {
	mdmirage.API
	days int
}

func (f *fakeMirageMD) AccumulateDayMax(context.Context) (int, error) { return f.days, nil }

type fakeMDMirage struct {
	masterdata.Reader
	mirage mdmirage.API
}

func (f *fakeMDMirage) Mirage() mdmirage.API { return f.mirage }

func clearedMirage() *gamestate.PlayerState {
	return &gamestate.PlayerState{ClearedQuests: map[int]struct{}{mirageUnlockQuestID: {}}}
}

func TestMirageFloorReceive(t *testing.T) {
	now := int64(1_700_000_000)
	// days=10 → 阈值 (10-1)*86400。fullTime 距 now 不足阈值 → 有可领。
	near := now + 5*secondsPerDay
	far := now + 20*secondsPerDay

	mk := func(state *gamestate.PlayerState, fullTime int64, received int) (*moduletest.FakeClient, *fakeMirageAPI) {
		api := &fakeMirageAPI{fullTime: fullTime, received: received}
		fc := &moduletest.FakeClient{
			State:         state,
			ServerTimeVal: now,
			MD:            &fakeMDMirage{mirage: &fakeMirageMD{days: 10}},
			MirageAPI:     api,
		}
		return fc, api
	}

	// 已解锁 + 池够 + 有奖励 → 领取成功。
	fc, api := mk(clearedMirage(), near, 3)
	if r := moduletest.RunOne(fc, mirageFloorReceive{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有奖励应成功: %+v", r)
	}
	if api.receives != 1 {
		t.Fatalf("应领取一次: %d", api.receives)
	}

	// 池未够（距注满仍久）→ skip，不领取。
	fc2, api2 := mk(clearedMirage(), far, 3)
	if r := moduletest.RunOne(fc2, mirageFloorReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("池未够应 skip: %+v", r)
	}
	if api2.receives != 0 {
		t.Fatal("池未够不应领取")
	}

	// 未解锁追忆战 → skip，不请求。
	fc3, api3 := mk(&gamestate.PlayerState{}, near, 3)
	if r := moduletest.RunOne(fc3, mirageFloorReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未解锁应 skip: %+v", r)
	}
	if api3.receives != 0 {
		t.Fatal("未解锁不应领取")
	}

	// 未通关（fullTime==-1）→ skip。
	fc4, _ := mk(clearedMirage(), -1, 3)
	if r := moduletest.RunOne(fc4, mirageFloorReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未通关应 skip: %+v", r)
	}
}
