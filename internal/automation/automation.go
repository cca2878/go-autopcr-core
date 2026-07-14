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

	"github.com/cca2878/go-autopcr-core/internal/client"
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

// Observation 是模块发射的一条结构化遥测观测（核心侧中性载荷）。核心不关心其去向：
// 缓冲/持久化/上传由外壳注入的 Collector 处理；具体线缆 schema 归遥测侧适配（core.Event →
// schema.Record）。Kind 为观测类别，Fields 为载荷（键名由各模块以常量约定）。
type Observation struct {
	Kind   string
	Fields map[string]any
}

// Collector 是 Run 的【只写遥测端口】：外壳注入，模块经 rc.Emit 同步推送 Observation。
// 契约同 Observer（尽快返回、不得 panic、勿依赖时序）。为 nil 时 Emit 为 no-op——不注入
// 采集器时模块行为不变、Run 返回的结果逐字节相同（确定性不受采集器影响）。
type Collector func(Observation)

// RunContext 是模块 Run 期间的上下文：提供本模块解析后的配置访问（嵌入 Config，故可直接
// rc.Bool/Int/String）、日志记录（rc.Logf）与遥测发射（rc.Emit）。后续如需扩展只在此累加。
type RunContext struct {
	Config
	rep       *Reporter
	collector Collector
}

// Logf 追加一行过程日志。
func (rc *RunContext) Logf(format string, args ...any) { rc.rep.Logf(format, args...) }

// Emit 向外壳注入的采集器推送一条遥测观测（未注入=no-op）。kind 为观测类别（如 "alces_roll"），
// fields 为载荷。发射是旁路：不影响模块的业务判定与 Run 结果。
func (rc *RunContext) Emit(kind string, fields map[string]any) {
	if rc.collector != nil {
		rc.collector(Observation{Kind: kind, Fields: fields})
	}
}

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

// Phase 标识进度事件在任务生命周期中的位置。
type Phase int

const (
	PhaseStarted  Phase = iota // 任务即将执行（尚无结果）
	PhaseFinished              // 任务已结束（Result 为已定稿结果的副本）
)

// Event 是一条【任务级】进度事件，按值传递。
type Event struct {
	Phase        Phase
	Index, Total int    // 第 Index（从 0 计）个任务，共 Total 个
	Meta         Meta   // 该任务模块的元信息（未知模块名时仅 Name）
	Result       Result // 仅 PhaseFinished 有意义：已定稿结果的独立副本
}

// Observer 是 Run 的【只写进度端口】：外壳注入，核心在任务边界【同步、按序推送】Event。
//
// 注入方须遵守（同步 push 固有）：
//   - 回调必须尽快返回、不得阻塞——它在 Run 的 goroutine 上同步调用，慢/阻塞会拖慢整条
//     任务链；耗时处理请自行转交其它线程。
//   - 不得 panic：进度上报是旁路，panic 会波及 Run（跨 FFI 时由 mobile skin 兜住异常）。
//   - 事件顺序确定（Started(i)→Finished(i) 依次），到达时刻不确定：勿让任何逻辑依赖时序。
//
// 核心侧保证（注入方无需操心）：Event 按值传递（含 Result 副本），核心绝不从 Observer 读回；
// 故 Observer 为 nil 与否，Run 返回的 []Result 逐字节相同——确定性不受观察者影响。
type Observer func(Event)

// Run 依次执行每个 Task 并返回对应结果：经 reg 把 Task.Module 解析为模块。单任务=长度 1 的
// 列表，批处理=多元素，二者走同一路径（统一单/批）。单个任务失败/跳过（含未知模块名）不影响
// 其余继续执行。
//
// 进度：obs 非 nil 时在每个任务前后推送 PhaseStarted / PhaseFinished（见 Observer 契约）；obs
// 为 nil 即无进度、行为与不传观察者完全一致。
//
// 取消（边界语义 / B1）：在开跑下一个任务前检查 ctx，已取消则【停止调度后续任务】，返回【已完成
// 部分】+ ctx.Err()。正在执行的任务因共享 ctx 被中断而失败时，归为取消而非失败——丢弃该结果、就地
// 停止。故返回的 error 非 nil 即“被取消，只跑了这些”，而结果里的 StatusError 永远只表示【真实失败】，
// 不含取消假象。
// col 是可选的遥测采集端口（见 Collector）：非 nil 时模块经 rc.Emit 推送的观测转交外壳；
// nil 即无遥测、行为与不传采集器完全一致。
func Run(ctx context.Context, gc client.GameClient, reg *Registry, tasks []Task, obs Observer, col Collector) ([]Result, error) {
	total := len(tasks)
	results := make([]Result, 0, total)
	for i, t := range tasks {
		// 边界取消：开跑下一个任务前检查，已取消则返回已完成部分与因由。
		if err := ctx.Err(); err != nil {
			return results, err
		}

		m, known := reg.Get(t.Module)
		meta := Meta{Name: t.Module}
		if known {
			meta = m.Meta()
		}
		emit(obs, Event{Phase: PhaseStarted, Index: i, Total: total, Meta: meta})

		var res Result
		if !known {
			res = Result{Meta: meta, Status: StatusError, Err: fmt.Errorf("未知模块 %q", t.Module)}
		} else {
			res = runOne(ctx, gc, m, t.Values, col)
			// 取消判定：任务因 ctx 取消被中断而失败时归为取消而非失败——丢弃结果、就地停止。
			if res.Status == StatusError && ctx.Err() != nil {
				return results, ctx.Err()
			}
		}

		results = append(results, res)
		emit(obs, Event{Phase: PhaseFinished, Index: i, Total: total, Meta: res.Meta, Result: cloneResult(res)})
	}
	return results, nil
}

// emit 向非 nil 的 obs 推送一条事件。
func emit(obs Observer, ev Event) {
	if obs != nil {
		obs(ev)
	}
}

// cloneResult 复制 Result（含 Log 切片）供事件按值携带，隔离于返回给调用方的结果。
func cloneResult(r Result) Result {
	if r.Log != nil {
		r.Log = append([]string(nil), r.Log...)
	}
	return r
}

func runOne(ctx context.Context, gc client.GameClient, m Module, values map[string]any, col Collector) Result {
	if err := Validate(m.Params(), values); err != nil {
		return Result{Meta: m.Meta(), Status: StatusError, Err: fmt.Errorf("配置无效: %w", err)}
	}
	rep := &Reporter{}
	rc := &RunContext{Config: resolve(m.Params(), values), rep: rep, collector: col}
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
