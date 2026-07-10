// Package daily 是「每日收取」域的游戏 API 能力面（礼物箱、任务奖励等；对应 ref daily.py）。
//
// 能力方法按具体系统分文件（present.go、mission.go…），共用同一 Impl（只持传输句柄）。
package daily

import (
	"context"

	"github.com/cca2878/go-autopcr/internal/client/internal/transport"
)

// API 是每日收取域能力面契约（随功能在本包内累加）。
type API interface {
	// PresentBox 拉取礼物箱内容（present/index）。
	PresentBox(ctx context.Context) ([]Present, error)
	// ReceiveAllPresents 领取礼物箱中符合条件的一批礼物。
	ReceiveAllPresents(ctx context.Context, excludeStamina bool) ([]Reward, error)
	// MissionList 拉取任务列表（mission/index），含各任务是否可领取。
	MissionList(ctx context.Context) ([]Mission, error)
	// AcceptMissions 领取某一类别（1/2/4）下全部可领取任务的奖励。
	AcceptMissions(ctx context.Context, category int) ([]Reward, error)
}

// Impl 是 API 的实现，只持有发请求所需的传输句柄。各系统的方法定义在各自文件中。
type Impl struct {
	tr *transport.Client
}

// New 构造每日收取域能力面实现。
func New(tr *transport.Client) *Impl { return &Impl{tr: tr} }
