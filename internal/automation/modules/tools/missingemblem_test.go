package tools

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	gapiemblem "github.com/cca2878/go-autopcr/internal/client/gameapi/emblem"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	mdemblem "github.com/cca2878/go-autopcr/internal/client/masterdata/emblem"
)

// fakeEmblemAPI mock 称号域能力面（已拥有称号）。
type fakeEmblemAPI struct {
	gapiemblem.API
	owned map[int]struct{}
}

func (f *fakeEmblemAPI) OwnedEmblemIDs(context.Context) (map[int]struct{}, error) {
	return f.owned, nil
}

// fakeEmblemMD mock 母数据称号域；fakeReader 覆写 Reader.Emblem()。
type fakeEmblemMD struct {
	mdemblem.API
	all []mdemblem.Emblem
}

func (f *fakeEmblemMD) AllEmblems(context.Context) ([]mdemblem.Emblem, error) { return f.all, nil }

type fakeReader struct {
	masterdata.Reader
	emblem mdemblem.API
}

func (f *fakeReader) Emblem() mdemblem.API { return f.emblem }

func TestMissingEmblem(t *testing.T) {
	all := []mdemblem.Emblem{
		{ID: 1, Name: "初心者", Description: "开始游戏"},
		{ID: 2, Name: "百战", Description: "通关100次"},
		{ID: 3, Name: "满图鉴"},
	}
	mk := func(ownedIDs ...int) *moduletest.FakeClient {
		owned := make(map[int]struct{}, len(ownedIDs))
		for _, id := range ownedIDs {
			owned[id] = struct{}{}
		}
		return &moduletest.FakeClient{
			State:     &gamestate.PlayerState{},
			MD:        &fakeReader{emblem: &fakeEmblemMD{all: all}},
			EmblemAPI: &fakeEmblemAPI{owned: owned},
		}
	}

	// 缺 2、3 → 成功。
	if r := moduletest.RunOne(mk(1), missingEmblem{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有缺称号应成功: %+v", r)
	}
	// 全拥有 → skip。
	if r := moduletest.RunOne(mk(1, 2, 3), missingEmblem{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("全称号应 skip: %+v", r)
	}
	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{State: &gamestate.PlayerState{}, MD: nil, EmblemAPI: &fakeEmblemAPI{}}
	if r := moduletest.RunOne(noMD, missingEmblem{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
