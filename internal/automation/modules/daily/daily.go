// Package daily 汇集「每日收取」域的自动化模块（礼物箱、任务奖励…）。
package daily

import "github.com/cca2878/go-autopcr/internal/automation"

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(presentReceive{})
	r.Register(missionReceive{})
	r.Register(jjcReward{})
	r.Register(charaFortune{})
	r.Register(seasonpassAccept{})
	r.Register(mirageFloorReceive{})
}
