// Package modules 装配默认注册表：聚合各【域子包】登记的具体自动化模块与批预设。
//
// 与框架分离：框架（Module/Runner/Registry/Config…）在 internal/automation；具体模块按游戏域
// 拆到子包（modules/account、modules/daily、modules/story…），每个子包内模块类型不导出、由其
// Register(r) 登记。本文件只做聚合，故新增域=建子包 + 在此加一行 Register，框架与模块都不淹没。
package modules

import (
	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/account"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/clan"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/daily"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/room"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/story"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/sweep"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/tools"
	"github.com/cca2878/go-autopcr-core/internal/automation/modules/unit"
)

// DefaultRegistry 返回内置模块与批预设的注册表。各域在其子包 Register 内登记，此处聚合。
func DefaultRegistry() *automation.Registry {
	r := automation.NewRegistry()
	account.Register(r)
	daily.Register(r)
	room.Register(r)
	clan.Register(r)
	story.Register(r)
	sweep.Register(r)
	tools.Register(r)
	unit.Register(r)
	r.RegisterPreset(automation.Preset{Name: "readonly", Title: "只读检查", Modules: []string{"summary", "home"}})
	return r
}
