// Package story 汇集"剧情"域的自动化模块（生日/主线/角色好感剧情的可读报告，只读）。
//
// 这些模块移植自参考项目的剧情阅读模块，但一律降级为只读查询/报告（不发 story/view 写请求，
// 不改动线上数据），用于验证 masterdata 分域查询设计。
package story

import (
	"fmt"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
)

// maxStoryReportTitles 限制报告里列出的篇名数量，避免一次性列出过长。
const maxStoryReportTitles = 20

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(birthdayStoryReport{})
	r.Register(mainStoryReport{})
	r.Register(unitStoryReport{})
}

// formatReadable 把可读篇名列表格式化为报告串（超上限则截断并计数）。
func formatReadable(titles []string) string {
	shown := titles
	suffix := ""
	if len(shown) > maxStoryReportTitles {
		shown = shown[:maxStoryReportTitles]
		suffix = fmt.Sprintf(" 等 %d 篇", len(titles))
	}
	return strings.Join(shown, "、") + suffix
}
