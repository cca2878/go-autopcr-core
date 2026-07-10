package room

import (
	"context"
	"fmt"
	"testing"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	mdunit "github.com/cca2878/go-autopcr/internal/client/masterdata/unit"
)

// fakeUnitMD mock 母数据角色域（亲密度上限 + 角色名）。
type fakeUnitMD struct {
	mdunit.API
	// totalByRarity 给出各星级的 (love_level, total_love) 上限。
	totalByRarity map[int][2]int
}

func (f *fakeUnitMD) MaxTotalLove(_ context.Context, rarity int) (int, int, error) {
	// 复刻 ref：取 rarity ≤ 给定值范围内的最大上限。
	best := [2]int{}
	for r, v := range f.totalByRarity {
		if r <= rarity && v[1] > best[1] {
			best = v
		}
	}
	return best[0], best[1], nil
}

func (f *fakeUnitMD) Name(_ context.Context, unitID int) (string, error) {
	return fmt.Sprintf("角色%d", unitID/100), nil
}

type fakeReader struct {
	masterdata.Reader
	unit mdunit.API
}

func (f *fakeReader) Unit() mdunit.API { return f.unit }

func TestLoveUpReport(t *testing.T) {
	md := &fakeReader{unit: &fakeUnitMD{totalByRarity: map[int][2]int{
		3: {8, 3000},  // 3 星上限亲密度 3000（love_level 8）
		5: {12, 8000}, // 5 星上限亲密度 8000
	}}}

	// 一个未满（3000<8000）+ 一个已满 → 报告 1 个。
	state := &gamestate.PlayerState{
		Units: []gamestate.OwnedUnit{
			{ID: 100101, Rarity: 5}, // chara 1001
			{ID: 100201, Rarity: 5}, // chara 1002
		},
		CharaLove: map[int]int{1001: 3000, 1002: 8000},
	}
	fc := &moduletest.FakeClient{State: state, MD: md}
	if r := moduletest.RunOne(fc, loveUpReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有未满角色应成功: %+v", r)
	}

	// 全部已满 → skip。
	full := &gamestate.PlayerState{
		Units:     []gamestate.OwnedUnit{{ID: 100101, Rarity: 5}},
		CharaLove: map[int]int{1001: 8000},
	}
	if r := moduletest.RunOne(&moduletest.FakeClient{State: full, MD: md}, loveUpReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("全满应 skip: %+v", r)
	}

	// 未启用母数据 → error。
	if r := moduletest.RunOne(&moduletest.FakeClient{State: full, MD: nil}, loveUpReport{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
