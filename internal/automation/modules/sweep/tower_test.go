package sweep

import (
	"context"
	"testing"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapitower "github.com/cca2878/go-autopcr-core/internal/client/gameapi/tower"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdtower "github.com/cca2878/go-autopcr-core/internal/client/masterdata/tower"
)

// fakeTowerAPI mock 露娜塔域能力面。
type fakeTowerAPI struct {
	gapitower.API
	top gapitower.Top
}

func (f *fakeTowerAPI) Top(context.Context) (gapitower.Top, error) { return f.top, nil }

// fakeTowerMD mock 母数据露娜塔域；fakeTowerReader 覆写 Reader.Tower()。
type fakeTowerMD struct {
	mdtower.API
	start, end time.Time
	ok         bool
}

func (f *fakeTowerMD) NewestWindow(context.Context) (time.Time, time.Time, bool, error) {
	return f.start, f.end, f.ok, nil
}

type fakeTowerReader struct {
	masterdata.Reader
	tower mdtower.API
}

func (f *fakeTowerReader) Tower() mdtower.API { return f.tower }

func clearedTower() *gamestate.PlayerState {
	return &gamestate.PlayerState{ClearedQuests: map[int]struct{}{towerUnlockQuestID: {}}}
}

func TestTowerCloisterReport(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	day := 24 * time.Hour
	openWindow := &fakeTowerMD{start: now.Add(-2 * day), end: now.Add(5 * day), ok: true}

	mk := func(state *gamestate.PlayerState, mdw *fakeTowerMD, top gapitower.Top) *moduletest.FakeClient {
		return &moduletest.FakeClient{
			State:         state,
			ServerTimeVal: now.Unix(),
			MD:            &fakeTowerReader{tower: mdw},
			TowerAPI:      &fakeTowerAPI{top: top},
		}
	}

	// 开放 + 首关已通 + 有剩余 → 成功。
	fc := mk(clearedTower(), openWindow, gapitower.Top{CloisterFirstCleared: true, CloisterRemainClear: 2})
	if r := moduletest.RunOne(fc, towerCloisterReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("可扫荡应成功: %+v", r)
	}

	// 已扫荡完 → skip。
	fcDone := mk(clearedTower(), openWindow, gapitower.Top{CloisterFirstCleared: true, CloisterRemainClear: 0})
	if r := moduletest.RunOne(fcDone, towerCloisterReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("已扫荡应 skip: %+v", r)
	}

	// 未开放窗口 → skip。
	closed := &fakeTowerMD{start: now.Add(5 * day), end: now.Add(9 * day), ok: true}
	fcClosed := mk(clearedTower(), closed, gapitower.Top{CloisterFirstCleared: true, CloisterRemainClear: 2})
	if r := moduletest.RunOne(fcClosed, towerCloisterReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未开放应 skip: %+v", r)
	}

	// 未解锁露娜塔 → skip。
	fcLocked := mk(&gamestate.PlayerState{}, openWindow, gapitower.Top{CloisterFirstCleared: true, CloisterRemainClear: 2})
	if r := moduletest.RunOne(fcLocked, towerCloisterReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未解锁应 skip: %+v", r)
	}
}
