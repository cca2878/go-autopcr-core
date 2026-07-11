package room

import (
	"context"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/moduletest"
	gapiroom "github.com/cca2878/go-autopcr-core/internal/client/gameapi/room"
)

// fakeRoom mock 家园域能力面。
type fakeRoom struct {
	gapiroom.API
	start      func(ctx context.Context) ([]gapiroom.Item, error)
	receiveAll func(ctx context.Context) ([]gapiroom.Reward, error)
}

func (f *fakeRoom) Start(ctx context.Context) ([]gapiroom.Item, error) { return f.start(ctx) }
func (f *fakeRoom) ReceiveAll(ctx context.Context) ([]gapiroom.Reward, error) {
	return f.receiveAll(ctx)
}

func TestRoomAccept(t *testing.T) {
	// 有待收产物 → 收取，成功。
	fc := &moduletest.FakeClient{RoomAPI: &fakeRoom{
		start: func(context.Context) ([]gapiroom.Item, error) {
			return []gapiroom.Item{{SerialID: 1, ItemCount: 0}, {SerialID: 2, ItemCount: 120}}, nil
		},
		receiveAll: func(context.Context) ([]gapiroom.Reward, error) {
			return []gapiroom.Reward{{Type: 3, ID: 90001, Count: 120}}, nil
		},
	}}
	if r := moduletest.RunOne(fc, roomAccept{}, nil); r.Status != automation.StatusOK {
		t.Fatalf("有待收产物应成功: %+v", r)
	}

	// 全部 item_count==0 → skip，不调用 receive_all。
	received := false
	empty := &moduletest.FakeClient{RoomAPI: &fakeRoom{
		start: func(context.Context) ([]gapiroom.Item, error) {
			return []gapiroom.Item{{SerialID: 1, ItemCount: 0}}, nil
		},
		receiveAll: func(context.Context) ([]gapiroom.Reward, error) { received = true; return nil, nil },
	}}
	if r := moduletest.RunOne(empty, roomAccept{}, nil); r.Status != automation.StatusSkip {
		t.Fatalf("无待收产物应 skip: %+v", r)
	}
	if received {
		t.Fatal("无待收产物不应调用 receive_all")
	}
}
