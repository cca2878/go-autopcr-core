package daily

import (
	"context"

	dailypb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/daily"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// 库存/物品领域常量：体力饮料 =（Stamina 类型, 93001）。
const (
	invTypeStamina = 6
	staminaDrinkID = 93001
)

// Present 是礼物箱中的一件礼物（含判定所需的类型/物品 id）。
type Present struct {
	PresentID   int
	RewardType  int
	RewardID    int
	RewardCount int
}

// IsStaminaDrink 判断该礼物是否为体力饮料（领取会增体力，满时溢出浪费）。
func (p Present) IsStaminaDrink() bool {
	return p.RewardType == invTypeStamina && p.RewardID == staminaDrinkID
}

// Reward 是一次领取到的物品：库存类型 + 物品 id + 数量。名称解析（依赖母数据）待后续增强。
type Reward struct {
	Type  int
	ID    int
	Count int
}

// PresentBox 拉取礼物箱内容（present/index）。供调用方先查后领——只在确有可领取礼物时才
// receive，避免触发「已领取」等业务错误。
func (a *Impl) PresentBox(ctx context.Context) ([]Present, error) {
	req := &dailypb.PresentIndexRequest{TimeFilter: -1, TypeFilter: 0, DescFlag: true, Offset: 0}
	resp, err := transport.Call[dailypb.PresentIndexResponse](ctx, a.tr, req)
	if err != nil {
		return nil, err
	}
	out := make([]Present, len(resp.PresentInfoList))
	for i, p := range resp.PresentInfoList {
		out[i] = Present{PresentID: p.PresentID, RewardType: p.RewardType, RewardID: p.RewardID, RewardCount: p.RewardCount}
	}
	return out, nil
}

// ReceiveAllPresents 领取礼物箱中符合条件的一批礼物（excludeStamina=true 时跳过体力饮料）。
// 礼物箱较大时游戏按批返回，需多次调用直到返回空。请求构造与发包在此完成。
func (a *Impl) ReceiveAllPresents(ctx context.Context, excludeStamina bool) ([]Reward, error) {
	req := &dailypb.PresentReceiveAllRequest{
		TimeFilter:       -1,
		TypeFilter:       0,
		DescFlag:         true,
		IsExcludeStamina: excludeStamina,
	}
	resp, err := transport.Call[dailypb.PresentReceiveAllResponse](ctx, a.tr, req)
	if err != nil {
		return nil, err
	}
	out := make([]Reward, len(resp.Rewards))
	for i, r := range resp.Rewards {
		out[i] = Reward{Type: r.Type, ID: r.ID, Count: r.Count}
	}
	return out, nil
}
