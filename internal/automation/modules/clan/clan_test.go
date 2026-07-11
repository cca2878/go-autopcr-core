package clan

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapiclan "github.com/cca2878/go-autopcr-core/internal/client/gameapi/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
)

// fakeClan mock 公会域能力面。
type fakeClan struct {
	gapiclan.API
	members func(ctx context.Context, clanID int64) ([]gapiclan.Member, error)
	like    func(ctx context.Context, clanID, targetViewerID int64) error
}

func (f *fakeClan) Members(ctx context.Context, clanID int64) ([]gapiclan.Member, error) {
	return f.members(ctx, clanID)
}
func (f *fakeClan) Like(ctx context.Context, clanID, targetViewerID int64) error {
	return f.like(ctx, clanID, targetViewerID)
}

func TestClanLike(t *testing.T) {
	members := []gapiclan.Member{{ViewerID: 100, Name: "我"}, {ViewerID: 200, Name: "队友A"}, {ViewerID: 300, Name: "队友B"}}

	// 未加入公会 → skip，不请求成员/点赞。
	touched := false
	noClan := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ViewerID: 100, ClanID: 0},
		ClanAPI: &fakeClan{
			members: func(context.Context, int64) ([]gapiclan.Member, error) { touched = true; return members, nil },
			like:    func(context.Context, int64, int64) error { touched = true; return nil },
		},
	}
	if r := moduletest.RunOne(noClan, clanLike{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未加入公会应 skip: %+v", r)
	}
	if touched {
		t.Fatal("未加入公会不应请求成员或点赞")
	}

	// 今日已点赞 → skip。
	touched = false
	already := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ViewerID: 100, ClanID: 9, ClanLikeCount: 1},
		ClanAPI: &fakeClan{
			members: func(context.Context, int64) ([]gapiclan.Member, error) { touched = true; return members, nil },
			like:    func(context.Context, int64, int64) error { touched = true; return nil },
		},
	}
	if r := moduletest.RunOne(already, clanLike{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("今日已点赞应 skip: %+v", r)
	}
	if touched {
		t.Fatal("今日已点赞不应请求成员或点赞")
	}

	// 正常：有其他成员 → 点赞其中一人（不点自己），成功。
	var liked int64
	ok := &moduletest.FakeClient{
		State: &gamestate.PlayerState{ViewerID: 100, ClanID: 9},
		ClanAPI: &fakeClan{
			members: func(context.Context, int64) ([]gapiclan.Member, error) { return members, nil },
			like:    func(_ context.Context, _ int64, target int64) error { liked = target; return nil },
		},
	}
	if r := moduletest.RunOne(ok, clanLike{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有其他成员应成功: %+v", r)
	}
	if liked != 200 && liked != 300 {
		t.Fatalf("应点赞其他成员之一（200/300），实际 %d", liked)
	}
}
