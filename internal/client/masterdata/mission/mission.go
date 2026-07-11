// Package mission 是母数据「任务」域的只读查询（任务归类等；随需增量）。
package mission

import (
	"context"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata/mddb"
)

// 任务领取类别（对应 mission/accept 的 type 参数；复刻 ref 的 1/2/4）。
const (
	CategoryDaily      = 1 // 日常任务（含女神祭 season_pack）
	CategoryStationary = 2 // 常驻任务
	CategoryEmblem     = 4 // 纹章任务
)

// API 是任务域查询契约（随功能在本包内累加）。
type API interface {
	// Classifier 加载任务分类所需的几张小表，返回按 mission_id 归类的分类器。
	Classifier(ctx context.Context) (*Classifier, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造任务域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

// Classifier 加载任务分类所需的几张母数据小表，返回按 mission_id 归类的分类器。
//
// 归类规则复刻 ref：日常=daily_mission_data ∪ season_pack(mission_id≠0)，常驻=
// stationary_mission_data，纹章=emblem_mission_data。
func (a *Impl) Classifier(ctx context.Context) (*Classifier, error) {
	c := &Classifier{
		daily:      map[int]struct{}{},
		stationary: map[int]struct{}{},
		emblem:     map[int]struct{}{},
	}
	if err := a.db.LoadIntSet(ctx, c.daily, "SELECT daily_mission_id FROM daily_mission_data"); err != nil {
		return nil, err
	}
	if err := a.db.LoadIntSet(ctx, c.daily, "SELECT mission_id FROM season_pack WHERE mission_id <> 0"); err != nil {
		return nil, err
	}
	if err := a.db.LoadIntSet(ctx, c.stationary, "SELECT stationary_mission_id FROM stationary_mission_data"); err != nil {
		return nil, err
	}
	if err := a.db.LoadIntSet(ctx, c.emblem, "SELECT mission_id FROM emblem_mission_data"); err != nil {
		return nil, err
	}
	return c, nil
}

// Classifier 按 mission_id 判定任务归属的领取类别。
//
// 领取接口 mission/accept 只接受类别 type（1/2/4）而非单个 mission_id，故需先据母数据把
// 待领任务归类，只对确有可领任务的类别发起领取，避免对空类别触发业务错误。
type Classifier struct {
	daily      map[int]struct{}
	stationary map[int]struct{}
	emblem     map[int]struct{}
}

// NewClassifier 用各类别的 mission_id 集合直接构造分类器（供测试）。
func NewClassifier(daily, stationary, emblem []int) *Classifier {
	toSet := func(ids []int) map[int]struct{} {
		s := make(map[int]struct{}, len(ids))
		for _, id := range ids {
			s[id] = struct{}{}
		}
		return s
	}
	return &Classifier{daily: toSet(daily), stationary: toSet(stationary), emblem: toSet(emblem)}
}

// Type 返回 mission_id 所属的领取类别；不属于任何已知类别返回 0。
func (c *Classifier) Type(missionID int) int {
	if _, ok := c.daily[missionID]; ok {
		return CategoryDaily
	}
	if _, ok := c.stationary[missionID]; ok {
		return CategoryStationary
	}
	if _, ok := c.emblem[missionID]; ok {
		return CategoryEmblem
	}
	return 0
}
