// Package labyrinth 是"黎明界（迷宫）"域的游戏 API 能力面（对应参考项目 labyrinth_top/enter/retire）。
//
// 刷开局：Enter 拉一张随机地图，模块判定路线是否满足条件，不满足则 Retire 重刷。是否解锁由上层据
// 玩家任务状态判定；本域只负责发包并把响应整理为域结果类型。
package labyrinth

import (
	"context"

	labyrinthpb "github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/labyrinth"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Block 是地图上的一个格子（域结果类型）。BlockType 为 eLabyrinthBlockType：
// 1 起点、2 普通怪、3 EX怪、4 角色、5 事件、6 遗物、7 商店、8 Boss。
type Block struct {
	Area            int
	Column          int
	Row             int
	BlockID         int
	BlockType       int
	QuestID         int
	NextBlockIDList []int
}

// TopResult 是黎明界首页状态：当前开局 id（0＝无）与各公会已通关难度值。
type TopResult struct {
	EnterID             int
	ClearedDifficulties []int
}

// EnterResult 是一次进入的结果：本局 id 与地图格子列表。
type EnterResult struct {
	EnterID int
	Blocks  []Block
}

// API 是黎明界域能力面契约（随功能在本包内累加）。
type API interface {
	// Top 拉取黎明界首页（当前开局 id 与已通关难度）。
	Top(ctx context.Context) (*TopResult, error)
	// Enter 以指定公会与难度进入（生成随机地图），返回本局 id 与地图。
	Enter(ctx context.Context, guildID, difficulty int) (*EnterResult, error)
	// Retire 撤退指定开局。
	Retire(ctx context.Context, enterID int) error
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。
type Impl struct {
	tr *transport.Client
}

// New 构造黎明界域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }

func (a *Impl) Top(ctx context.Context) (*TopResult, error) {
	resp, err := transport.Call[labyrinthpb.TopResponse](ctx, a.tr, &labyrinthpb.TopRequest{})
	if err != nil {
		return nil, err
	}
	var cleared []int
	for _, info := range resp.GuildClearedDifficultyList {
		if info.Difficulty != 0 {
			cleared = append(cleared, info.Difficulty)
		}
	}
	return &TopResult{EnterID: resp.EnterID, ClearedDifficulties: cleared}, nil
}

func (a *Impl) Enter(ctx context.Context, guildID, difficulty int) (*EnterResult, error) {
	resp, err := transport.Call[labyrinthpb.EnterResponse](ctx, a.tr, &labyrinthpb.EnterRequest{GuildID: guildID, Difficulty: difficulty})
	if err != nil {
		return nil, err
	}
	blocks := make([]Block, len(resp.MapList))
	for i, m := range resp.MapList {
		blocks[i] = Block{
			Area:            m.Area,
			Column:          m.Column,
			Row:             m.Row,
			BlockID:         m.BlockID,
			BlockType:       m.BlockType,
			QuestID:         m.QuestID,
			NextBlockIDList: m.NextBlockIDList,
		}
	}
	return &EnterResult{EnterID: resp.EnterID, Blocks: blocks}, nil
}

func (a *Impl) Retire(ctx context.Context, enterID int) error {
	_, err := transport.Call[labyrinthpb.RetireResponse](ctx, a.tr, &labyrinthpb.RetireRequest{EnterID: enterID})
	return err
}
