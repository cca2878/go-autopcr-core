// Package tools 汇集「查询工具」域的自动化模块（图鉴/缺口类只读报告；对应 ref tools.py）。
package tools

import "github.com/cca2878/go-autopcr-core/internal/automation"

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(missingEmblem{})
	r.Register(exEquipInfo{})
	r.Register(halfMonth{})
}
