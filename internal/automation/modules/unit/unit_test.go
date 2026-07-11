package unit

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
	mdunit "github.com/cca2878/go-autopcr-core/internal/client/masterdata/unit"
)

// fakeUnitData mock 母数据角色域；fakeReader 覆写访问器 Unit()。
type fakeUnitData struct {
	mdunit.API
	obtainables func(ctx context.Context) ([]mdunit.Obtainable, error)
}

func (f *fakeUnitData) Obtainables(ctx context.Context) ([]mdunit.Obtainable, error) {
	return f.obtainables(ctx)
}

type fakeReader struct {
	masterdata.Reader
	unit mdunit.API
}

func (f *fakeReader) Unit() mdunit.API { return f.unit }

func TestReturnJewel(t *testing.T) {
	// 无角色 → skip。
	if r := moduletest.RunOne(&moduletest.FakeClient{State: &gamestate.PlayerState{}}, returnJewel{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无角色应 skip: %+v", r)
	}
	// 有角色 → 成功输出。
	units := make([]gamestate.OwnedUnit, 25)
	for i := range units {
		units[i] = gamestate.OwnedUnit{ID: 100101 + i, Level: 100 + i, PromotionLevel: 10, Rarity: 5}
	}
	fc := &moduletest.FakeClient{State: &gamestate.PlayerState{Units: units}}
	if r := moduletest.RunOne(fc, returnJewel{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有角色应成功: %+v", r)
	}
}

func TestMissingUnit(t *testing.T) {
	obtainable := []mdunit.Obtainable{
		{UnitID: 100101, Name: "日和莉", IsLimited: false},
		{UnitID: 100201, Name: "优衣", IsLimited: false},
		{UnitID: 100801, Name: "铃奈(泳装)", IsLimited: true},
	}
	mkClient := func(ownedIDs ...int) *moduletest.FakeClient {
		units := make([]gamestate.OwnedUnit, len(ownedIDs))
		for i, id := range ownedIDs {
			units[i] = gamestate.OwnedUnit{ID: id}
		}
		return &moduletest.FakeClient{
			State: &gamestate.PlayerState{Units: units},
			MD:    &fakeReader{unit: &fakeUnitData{obtainables: func(context.Context) ([]mdunit.Obtainable, error) { return obtainable, nil }}},
		}
	}

	// 持有日和莉 → 缺 优衣(常驻) + 铃奈(限定)。
	if r := moduletest.RunOne(mkClient(100101), missingUnit{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有缺角色应成功: %+v", r)
	}
	// 全持有 → skip。
	if r := moduletest.RunOne(mkClient(100101, 100201, 100801), missingUnit{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("全图鉴应 skip: %+v", r)
	}
	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: nil}
	if r := moduletest.RunOne(noMD, missingUnit{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
