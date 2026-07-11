// Package dungeon 是「地下城」域的游戏 API 能力面（dungeon/info；对应 ref underground 扫荡）。
package dungeon

import (
	"context"

	dungeonpb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/dungeon"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Info 是地下城当前状态摘要（供扫荡/报告判定）。
type Info struct {
	EnterAreaID   int // 当前所在区域 id（0＝未进入）
	RestChallenge int // 剩余挑战次数（各类合计）
	MaxChallenge  int // 挑战次数上限（各类合计）
}

// API 是地下城域能力面契约（随功能在本包内累加）。
type API interface {
	// Info 返回地下城当前状态（所在区域、剩余/上限挑战次数）。
	Info(ctx context.Context) (Info, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造地下城域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Info(ctx context.Context) (Info, error) {
	resp, err := transport.Call[dungeonpb.DungeonInfoResponse](ctx, a.tr, &dungeonpb.DungeonInfoRequest{})
	if err != nil {
		return Info{}, err
	}
	info := Info{EnterAreaID: resp.EnterAreaID}
	for _, r := range resp.RestChallengeCount {
		info.RestChallenge += r.Count
		info.MaxChallenge += r.MaxCount
	}
	return info, nil
}
