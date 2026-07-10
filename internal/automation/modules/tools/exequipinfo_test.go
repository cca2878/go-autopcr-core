package tools

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	mdexequip "github.com/cca2878/go-autopcr/internal/client/masterdata/exequip"
)

// fakeExequipMD mock 母数据 EX 装备域；fakeExReader 覆写 Reader.Exequip()。
type fakeExequipMD struct {
	mdexequip.API
	rarity map[int]int
}

func (f *fakeExequipMD) RarityByID(context.Context) (map[int]int, error) { return f.rarity, nil }

type fakeExReader struct {
	masterdata.Reader
	exequip mdexequip.API
}

func (f *fakeExReader) Exequip() mdexequip.API { return f.exequip }

func TestExEquipInfo(t *testing.T) {
	md := &fakeExReader{exequip: &fakeExequipMD{rarity: map[int]int{
		101: 5, 102: 5, 201: 4, 301: 3,
	}}}

	// 有 EX 装备 → 计数成功。
	fc := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ExEquipIDs: []int{101, 102, 201, 301, 999}}, // 999 未知→其他
		MD:    md,
	}
	if r := moduletest.RunOne(fc, exEquipInfo{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有 EX 装备应成功: %+v", r)
	}

	// 无 EX 装备 → skip。
	if r := moduletest.RunOne(&moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: md}, exEquipInfo{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无 EX 装备应 skip: %+v", r)
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{ExEquipIDs: []int{101}}, MD: nil}
	if r := moduletest.RunOne(noMD, exEquipInfo{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
