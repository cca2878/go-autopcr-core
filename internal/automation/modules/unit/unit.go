// Package unit 汇集"角色/图鉴"域的只读报告模块（练度返钻、图鉴缺口…）。
package unit

import "github.com/cca2878/go-autopcr-core/internal/automation"

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(returnJewel{})
	r.Register(missingUnit{})
}
