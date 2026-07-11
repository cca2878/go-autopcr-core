package sweep

import (
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
)

func TestExploreExpReport(t *testing.T) {
	// 有剩余次数 → 报告成功。
	fc := &moduletest.FakeClient{State: &gamestate.PlayerState{TrainingExpDone: 1, TrainingExpMax: 3}}
	if r := moduletest.RunOne(fc, exploreExpReport{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有剩余应成功: %+v", r)
	}

	// 已扫荡完 → skip。
	done := &moduletest.FakeClient{State: &gamestate.PlayerState{TrainingExpDone: 3, TrainingExpMax: 3}}
	if r := moduletest.RunOne(done, exploreExpReport{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("已扫荡完应 skip: %+v", r)
	}
}
