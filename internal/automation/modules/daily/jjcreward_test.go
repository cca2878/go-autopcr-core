package daily

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapiarena "github.com/cca2878/go-autopcr-core/internal/client/gameapi/arena"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
)

// fakeArena mock 竞技场域能力面，记录领取调用。
type fakeArena struct {
	gapiarena.API
	arenaCount, grandCount int
	arenaRecv, grandRecv   bool
}

func (f *fakeArena) ArenaRewardCount(context.Context) (int, error)      { return f.arenaCount, nil }
func (f *fakeArena) GrandArenaRewardCount(context.Context) (int, error) { return f.grandCount, nil }
func (f *fakeArena) ReceiveArenaReward(context.Context) error           { f.arenaRecv = true; return nil }
func (f *fakeArena) ReceiveGrandArenaReward(context.Context) error      { f.grandRecv = true; return nil }

func clearedState(ids ...int) *gamestate.PlayerState {
	cleared := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		cleared[id] = struct{}{}
	}
	return &gamestate.PlayerState{ClearedQuests: cleared}
}

func TestJJCReward(t *testing.T) {
	// 两场均解锁且有可领 → 领取两场，成功。
	fa := &fakeArena{arenaCount: 3, grandCount: 5}
	fc := &moduletest.FakeClient{State: clearedState(arenaUnlockQuestID, grandArenaUnlockQuestID), ArenaAPI: fa}
	if r := moduletest.RunOne(fc, jjcReward{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有双场币应成功: %+v", r)
	}
	if !fa.arenaRecv || !fa.grandRecv {
		t.Fatalf("应领取两场: arena=%v grand=%v", fa.arenaRecv, fa.grandRecv)
	}

	// 均解锁但无可领 → skip，不领取。
	fa2 := &fakeArena{arenaCount: 0, grandCount: 0}
	fc2 := &moduletest.FakeClient{State: clearedState(arenaUnlockQuestID, grandArenaUnlockQuestID), ArenaAPI: fa2}
	if r := moduletest.RunOne(fc2, jjcReward{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无可领应 skip: %+v", r)
	}
	if fa2.arenaRecv || fa2.grandRecv {
		t.Fatal("无可领不应领取")
	}

	// 未解锁竞技场 → 不查/不领该场（此处两场均未解锁）→ skip。
	fa3 := &fakeArena{arenaCount: 9, grandCount: 9}
	fc3 := &moduletest.FakeClient{State: clearedState(), ArenaAPI: fa3}
	if r := moduletest.RunOne(fc3, jjcReward{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("未解锁应 skip: %+v", r)
	}
	if fa3.arenaRecv || fa3.grandRecv {
		t.Fatal("未解锁不应领取")
	}
}
