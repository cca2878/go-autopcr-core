package clan

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	gapicb "github.com/cca2878/go-autopcr/internal/client/gameapi/clanbattle"
	"github.com/cca2878/go-autopcr/internal/client/gamestate"
)

// fakeClanBattle mock 公会战域能力面。
type fakeClanBattle struct {
	gapicb.API
	top   gapicb.Top
	calls int
}

func (f *fakeClanBattle) Top(context.Context, int64) (gapicb.Top, error) {
	f.calls++
	return f.top, nil
}

func TestClanBattleKnive(t *testing.T) {
	// 未加入公会 → skip，不请求。
	fb := &fakeClanBattle{}
	noClan := &moduletest.FakeClient{State: &gamestate.PlayerState{ClanID: 0}, ClanBattleAPI: fb}
	if r := moduletest.RunOne(noClan, clanBattleKnive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无公会应 skip: %+v", r)
	}
	if fb.calls != 0 {
		t.Fatal("无公会不应请求 clan_battle/top")
	}

	// 有剩余刀 → 报告成功。
	fb2 := &fakeClanBattle{top: gapicb.Top{RemainingCount: 2, Point: 300, CarryOverTimes: []int{50}}}
	fc := &moduletest.FakeClient{State: &gamestate.PlayerState{ClanID: 123}, ClanBattleAPI: fb2}
	if r := moduletest.RunOne(fc, clanBattleKnive{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有刀应成功: %+v", r)
	}

	// 三刀出完（无剩余、满点、无尾刀）→ skip。
	fb3 := &fakeClanBattle{top: gapicb.Top{RemainingCount: 0, Point: clanBattlePointFull}}
	done := &moduletest.FakeClient{State: &gamestate.PlayerState{ClanID: 123}, ClanBattleAPI: fb3}
	if r := moduletest.RunOne(done, clanBattleKnive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("三刀出完应 skip: %+v", r)
	}
}
