// Package automation 是单账号自动化层（S3 运行器缝）——它「操作」无头客户端(S2)执行
// 一组任务模块。
//
// 模块只依赖 client.GameClient 的公共能力面（请求体构造与发包由客户端内部完成），不接触
// 其内部实现/协议，故本层可用 mock 独立测试。持久化/历史属上层职责，本层只产出结果。
package automation

import (
	"context"
	"errors"
	"fmt"

	"github.com/cca2878/go-autopcr/internal/client"
)

// Status 是单个模块的执行结果状态。
type Status string

const (
	StatusOK    Status = "ok"    // 正常执行
	StatusSkip  Status = "skip"  // 前置条件不满足，主动跳过
	StatusError Status = "error" // 执行出错
)

// Meta 是模块的静态元信息。
type Meta struct {
	Name        string // 稳定键（CLI/配置引用），如 "home"
	Title       string // 展示名，如 "刷新首页"
	Description string
	Category    string // 分组/批处理选择用
	// NeedsMasterdata 表示该模块运行时依赖母数据只读句柄（gc.Masterdata() 非 nil）。
	// 上层据此为选中含该标志的模块的运行启用 WithMasterdata。
	NeedsMasterdata bool
}

// Result 是单个模块运行后的结构化结果（不含持久化，交由上层处理）。
type Result struct {
	Meta   Meta
	Status Status
	Log    []string // 人类可读的过程/结论行
	Err    error    // Status==StatusError 时设置
}

// Reporter 供模块在 Run 期间记录过程行。
type Reporter struct {
	lines []string
}

// Logf 追加一行过程日志。
func (r *Reporter) Logf(format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

// RunContext 是模块 Run 期间的上下文：提供本模块解析后的配置访问（嵌入 Config，故可直接
// rc.Bool/Int/String）与日志记录（rc.Logf）。后续如需扩展只在此累加，不再改 Run 签名。
type RunContext struct {
	Config
	rep *Reporter
}

// Logf 追加一行过程日志。
func (rc *RunContext) Logf(format string, args ...any) { rc.rep.Logf(format, args...) }

// Module 是一个自动化任务单元：操作客户端能力面完成一件事。
type Module interface {
	Meta() Meta
	// Params 声明可配置参数（无则返回 nil）。
	Params() []Param
	// Run 执行任务：返回 nil=成功；返回 Skip(...)=前置不满足而跳过；其它 error=失败。
	Run(ctx context.Context, gc client.GameClient, rc *RunContext) error
}

// skipError 表示「主动跳过」，由 Skip 构造，Run 据此区分跳过与失败。
type skipError struct{ reason string }

func (e *skipError) Error() string { return e.reason }

// Skip 构造一个「跳过」信号（携带原因），供模块在前置条件不满足时返回。
func Skip(format string, args ...any) error {
	return &skipError{reason: fmt.Sprintf(format, args...)}
}

// Task 是一次待执行的任务的【纯数据】描述：模块标识符（Registry 中的名字）+ 该实例的原始
// 配置值（未 resolve，nil=全默认）。以 Task 为单位（而非直接持模块），故同一模块可在一批内
// 重复并各带不同配置；且 Task 可序列化（配置驱动的批定义），执行时经 Registry 解析名字。
type Task struct {
	Module string
	Values map[string]any
}

// TasksFor 为选中的模块各配一份来自 src 的配置（按模块名取值），产出 []Task——CLI 常见用法。
// 需要同模块多实例、各带不同配置时，直接构造 []Task 即可。
func TasksFor(mods []Module, src Source) []Task {
	tasks := make([]Task, len(mods))
	for i, m := range mods {
		name := m.Meta().Name
		tasks[i] = Task{Module: name, Values: src[name]}
	}
	return tasks
}

// Run 依次执行每个 Task 并返回对应结果：经 reg 把 Task.Module 解析为模块。单任务=长度 1 的
// 列表，批处理=多元素，二者走同一路径（统一单/批）。单个任务失败/跳过（含未知模块名）不影响
// 其余继续执行。
func Run(ctx context.Context, gc client.GameClient, reg *Registry, tasks []Task) []Result {
	results := make([]Result, 0, len(tasks))
	for _, t := range tasks {
		m, ok := reg.Get(t.Module)
		if !ok {
			results = append(results, Result{
				Meta:   Meta{Name: t.Module},
				Status: StatusError,
				Err:    fmt.Errorf("未知模块 %q", t.Module),
			})
			continue
		}
		results = append(results, runOne(ctx, gc, m, t.Values))
	}
	return results
}

func runOne(ctx context.Context, gc client.GameClient, m Module, values map[string]any) Result {
	if err := Validate(m.Params(), values); err != nil {
		return Result{Meta: m.Meta(), Status: StatusError, Err: fmt.Errorf("配置无效: %w", err)}
	}
	rep := &Reporter{}
	rc := &RunContext{Config: resolve(m.Params(), values), rep: rep}
	err := m.Run(ctx, gc, rc)
	res := Result{Meta: m.Meta(), Log: rep.lines}
	switch {
	case err == nil:
		res.Status = StatusOK
	case isSkip(err):
		res.Status = StatusSkip
		res.Log = append(res.Log, err.Error())
	default:
		res.Status = StatusError
		res.Err = err
	}
	return res
}

func isSkip(err error) bool {
	var se *skipError
	return errors.As(err, &se)
}
