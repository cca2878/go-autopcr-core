package daily

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapidaily "github.com/cca2878/go-autopcr-core/internal/client/gameapi/daily"
)

// fakeDaily mock 每日收取域能力面（present 相关）。
type fakeDaily struct {
	gapidaily.API
	box        func(ctx context.Context) ([]gapidaily.Present, error)
	receiveAll func(ctx context.Context, excludeStamina bool) ([]gapidaily.Reward, error)
}

func (f *fakeDaily) PresentBox(ctx context.Context) ([]gapidaily.Present, error) { return f.box(ctx) }
func (f *fakeDaily) ReceiveAllPresents(ctx context.Context, ex bool) ([]gapidaily.Reward, error) {
	return f.receiveAll(ctx, ex)
}

func TestPresentReceive(t *testing.T) {
	staminaOnly := []gapidaily.Present{{PresentID: 1, RewardType: 6, RewardID: 93001, RewardCount: 30}}
	realGift := []gapidaily.Present{{PresentID: 2, RewardType: 4, RewardID: 90005, RewardCount: 100}}

	// 有非体力礼物 → 领取（第一批领到，第二批空箱结束），成功。
	boxCalls := 0
	fc := &moduletest.FakeClient{DailyAPI: &fakeDaily{
		box: func(context.Context) ([]gapidaily.Present, error) {
			boxCalls++
			if boxCalls == 1 {
				return realGift, nil
			}
			return nil, nil
		},
		receiveAll: func(context.Context, bool) ([]gapidaily.Reward, error) {
			return []gapidaily.Reward{{Type: 4, ID: 90005, Count: 100}}, nil
		},
	}}
	if r := moduletest.RunOne(fc, presentReceive{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有礼物应成功: %+v", r)
	}

	// 空礼物箱 → skip，不调用 receive。
	received := false
	empty := &moduletest.FakeClient{DailyAPI: &fakeDaily{
		box:        func(context.Context) ([]gapidaily.Present, error) { return nil, nil },
		receiveAll: func(context.Context, bool) ([]gapidaily.Reward, error) { received = true; return nil, nil },
	}}
	if r := moduletest.RunOne(empty, presentReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("空箱应 skip: %+v", r)
	}
	if received {
		t.Fatal("空箱不应调用 receive")
	}

	// 仅体力礼物 + 默认排除 → skip，不调用 receive。
	received = false
	staminaBox := &moduletest.FakeClient{DailyAPI: &fakeDaily{
		box:        func(context.Context) ([]gapidaily.Present, error) { return staminaOnly, nil },
		receiveAll: func(context.Context, bool) ([]gapidaily.Reward, error) { received = true; return nil, nil },
	}}
	if r := moduletest.RunOne(staminaBox, presentReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("仅体力+默认排除应 skip: %+v", r)
	}
	if received {
		t.Fatal("仅体力+排除不应调用 receive")
	}
}
