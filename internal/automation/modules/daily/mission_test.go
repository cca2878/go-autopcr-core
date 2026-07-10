package daily

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr/internal/automation"
	"github.com/cca2878/go-autopcr/internal/automation/modules/moduletest"
	gapidaily "github.com/cca2878/go-autopcr/internal/client/gameapi/daily"
	"github.com/cca2878/go-autopcr/internal/client/masterdata"
	"github.com/cca2878/go-autopcr/internal/client/masterdata/mission"
)

// fakeMissionDaily mock 每日收取域能力面（mission 相关）。
type fakeMissionDaily struct {
	gapidaily.API
	list   func(ctx context.Context) ([]gapidaily.Mission, error)
	accept func(ctx context.Context, category int) ([]gapidaily.Reward, error)
}

func (f *fakeMissionDaily) MissionList(ctx context.Context) ([]gapidaily.Mission, error) {
	return f.list(ctx)
}
func (f *fakeMissionDaily) AcceptMissions(ctx context.Context, category int) ([]gapidaily.Reward, error) {
	return f.accept(ctx, category)
}

// fakeMissionData mock 母数据任务域；fakeReader 覆写访问器 Mission()。
type fakeMissionData struct {
	mission.API
	classify func(ctx context.Context) (*mission.Classifier, error)
}

func (f *fakeMissionData) Classifier(ctx context.Context) (*mission.Classifier, error) {
	return f.classify(ctx)
}

type fakeReader struct {
	masterdata.Reader
	mission mission.API
}

func (f *fakeReader) Mission() mission.API { return f.mission }

func TestMissionReceive(t *testing.T) {
	cls := mission.NewClassifier([]int{11}, []int{22}, []int{44})
	classifier := func(context.Context) (*mission.Classifier, error) { return cls, nil }
	mkReader := func() masterdata.Reader { return &fakeReader{mission: &fakeMissionData{classify: classifier}} }

	// 有可领取的日常+纹章（常驻不可领）→ 只对 1、4 领取（去重、升序、跳过 2）。
	var accepted []int
	fc := &moduletest.FakeClient{
		MD: mkReader(),
		DailyAPI: &fakeMissionDaily{
			list: func(context.Context) ([]gapidaily.Mission, error) {
				return []gapidaily.Mission{
					{ID: 11, Receivable: true},
					{ID: 22, Receivable: false},
					{ID: 44, Receivable: true},
					{ID: 44, Receivable: true},
				}, nil
			},
			accept: func(_ context.Context, cat int) ([]gapidaily.Reward, error) {
				accepted = append(accepted, cat)
				return []gapidaily.Reward{{Type: 4, ID: 90005, Count: 1}}, nil
			},
		},
	}
	if r := moduletest.RunOne(fc, missionReceive{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有可领任务应成功: %+v", r)
	}
	if len(accepted) != 2 || accepted[0] != 1 || accepted[1] != 4 {
		t.Fatalf("应按类别 [1 4] 升序领取，实际 %v", accepted)
	}

	// 无可领取任务 → skip，不领取。
	acceptCalled := false
	none := &moduletest.FakeClient{
		MD: mkReader(),
		DailyAPI: &fakeMissionDaily{
			list: func(context.Context) ([]gapidaily.Mission, error) {
				return []gapidaily.Mission{{ID: 11, Receivable: false}}, nil
			},
			accept: func(context.Context, int) ([]gapidaily.Reward, error) { acceptCalled = true; return nil, nil },
		},
	}
	if r := moduletest.RunOne(none, missionReceive{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无可领任务应 skip: %+v", r)
	}
	if acceptCalled {
		t.Fatal("无可领任务不应调用 accept")
	}

	// 未启用母数据 → error。
	noMD := &moduletest.FakeClient{
		MD: nil,
		DailyAPI: &fakeMissionDaily{
			list:   func(context.Context) ([]gapidaily.Mission, error) { return nil, nil },
			accept: func(context.Context, int) ([]gapidaily.Reward, error) { return nil, nil },
		},
	}
	if r := moduletest.RunOne(noMD, missionReceive{}, nil); r.Status != automation.StatusError {
		t.Fatalf("未启用母数据应 error: %+v", r)
	}
}
