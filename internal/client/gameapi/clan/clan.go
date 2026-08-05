// Package clan 是"公会"域的游戏 API 能力面（公会信息、点赞等；对应参考项目 clan.py）。
//
// 能力方法按具体系统分文件，共用同一 Impl（只持传输句柄）。
package clan

import (
	"context"

	clanpb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/clan"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// API 是公会域能力面契约（随功能在本包内累加）。
type API interface {
	// Members 返回指定公会的成员列表（供先查后动，如挑选点赞对象）。
	Members(ctx context.Context, clanID int64) ([]Member, error)
	// Like 为指定成员点赞。
	Like(ctx context.Context, clanID, targetViewerID int64) error
}

// Member 是公会一名成员（点赞所需字段）。
type Member struct {
	ViewerID int64
	Name     string
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造公会域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Members(ctx context.Context, clanID int64) ([]Member, error) {
	resp, err := transport.Call[clanpb.ClanInfoResponse](ctx, a.tr, &clanpb.ClanInfoRequest{ClanID: clanID, GetUserEquip: 0})
	if err != nil {
		return nil, err
	}
	if resp.Clan == nil {
		return nil, nil
	}
	out := make([]Member, len(resp.Clan.Members))
	for i, m := range resp.Clan.Members {
		out[i] = Member{ViewerID: m.ViewerID, Name: m.Name}
	}
	return out, nil
}

func (a *Impl) Like(ctx context.Context, clanID, targetViewerID int64) error {
	_, err := transport.Call[clanpb.ClanLikeResponse](ctx, a.tr, &clanpb.ClanLikeRequest{ClanID: clanID, TargetViewerID: targetViewerID})
	return err
}
