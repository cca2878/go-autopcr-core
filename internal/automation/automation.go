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
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
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

// Candidates 是 Module 的【可选】扩展：声明那些候选依赖世界（母数据 / 账号态）的参数如何解析。
//
// 为什么要它：Params() 是纯静态声明、够不着 gc，故只表达得了编译期就固定的候选。而「炼成哪件
// 彩装」这类参数的候选是玩家库存——登录后才知道。没有这个口子，这类参数只能退化成不受约束的
// 自由文本，选单与校验一起失去。
//
// 契约：只读【已有的世界】（母数据库 + gc.Data() 的玩家态快照），【不发新的网络请求】。满足
// 这条，它读的就是 Run 本就依赖的同一个世界，不给确定性引入新的隐藏输入。
//
// 返回「参数名 → 候选」。一次调用可服务多个参数，同一次母数据加载因此能摊给它们（如彩装模块
// 的四个副属性槽与「炼成哪件」共用一次快照）。参数名须已在 Params() 声明；每个无静态候选的
// Choice 类参数都须在此给出候选（空切片＝世界里当前没有可选项，合法）——两条都由
// bindCandidates 强制。
type Candidates interface {
	Candidates(ctx context.Context, gc client.GameClient) (map[string][]Option, error)
}

// BreakPolicy 是模块对「执行到一半会话被顶掉、客户端已重登」的处置声明。别名自 client 包
// （策略要传到传输层才起作用），模块只需用这里的名字。
type BreakPolicy = client.BreakPolicy

const (
	// BreakAbort 是默认（不实现 SessionAware 即此值）：断点处当场失败，任务报错、交用户重跑。
	BreakAbort = client.BreakAbort
	// BreakRestart 表示本模块从头重跑一遍即可，运行器会在断点后自动重跑一次。
	BreakRestart = client.BreakRestart
	// BreakIgnore 表示断点无所谓，会话错误照常自愈重发，模块无感。
	BreakIgnore = client.BreakIgnore
)

// SessionAware 是 Module 的【可选】扩展：声明模块如何应对执行期间的会话断点。
//
// 为什么要它：会话失效（被其他客户端顶号、数据不一致）时客户端会自动重登，但重登只修得了
// 会话——修不了模块【已经查到、存在局部变量里】的那份世界快照。「刚查到礼物箱有 3 件」在
// 断点之后可能已经不成立，而框架看不见这些局部变量，无从校正。故断点后怎么办只有模块自己
// 知道，这里是它表态的唯一位置。
//
// 不实现即 BreakAbort——最保守的一档：宁可让任务失败让用户重跑，也不拿旧世界的结论去写新世界。
//
// 选 BreakRestart 前请确认【重跑一遍不会重复扣资源】：这正是「先查后动」铁律的红利——收取类
// 模块重跑时会先查、发现已领完即无操作。若模块按次数循环消耗（如按配置扫荡 N 次），重跑就是
// 又扣 N 次，那它【不能】声明 BreakRestart。
// 选 BreakIgnore 请确认模块【完全不写入】（纯查询/报告）：此档下断点被静默重发掩盖，模块会
// 拿着可能过时的快照继续跑完。
//
// 另注：BreakRestart 会把断点前 rc.Emit 过的观测【再发一遍】（遥测按次计），声明前一并考虑。
// 别按 Meta.Category 反推本策略——那是展示用的分组字符串，不是「是否写入」的契约。
type SessionAware interface {
	OnSessionBreak() BreakPolicy
}

// breakPolicyOf 取模块声明的断点策略；未实现 SessionAware 即最保守的 BreakAbort。
func breakPolicyOf(m Module) BreakPolicy {
	if s, ok := m.(SessionAware); ok {
		return s.OnSessionBreak()
	}
	return BreakAbort
}

// CheckCandidates 在给定世界下解析模块的参数候选并报告其是否自洽，供模块单测做契约检查——
// runOne 每次运行都做同样的解析，故它就是「这个模块跑起来会不会因参数候选而失败」的提前问询。
//
// Registry.Register 只抓得住「整个 Candidates 接口都没实现」（无需世界即可判定）；漏掉其中
// 【某一个】参数则要真解析一次才知道，那正是本函数的位置。gc 用模块单测现成的假客户端即可。
func CheckCandidates(ctx context.Context, gc client.GameClient, m Module) error {
	_, err := resolveParams(ctx, gc, m)
	return err
}

// resolveParams 取模块的参数定义，并在其实现了 Candidates 时解析依赖世界的候选、填进 Bounds。
func resolveParams(ctx context.Context, gc client.GameClient, m Module) ([]Param, error) {
	var cands map[string][]Option
	if c, ok := m.(Candidates); ok {
		var err error
		if cands, err = c.Candidates(ctx, gc); err != nil {
			return nil, err
		}
	}
	return bindCandidates(m.Params(), cands)
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
	// 把模块的会话断点策略交给传输层：断点是在某次 gc 调用【里面】被发现的，只有那里能当场
	// 中止（而不是等模块跑完再秋后算账，那时旧世界的结论早已写进新世界）。见 SessionAware。
	policy := breakPolicyOf(m)
	ctx = client.WithBreakPolicy(ctx, policy)

	// 过程日志跨重跑保留：断点前那半程也是用户要看的（尤其它可能已经写入过）。
	rep := &Reporter{}
	for attempt := 0; ; attempt++ {
		// 先按【当前世界】把依赖它的候选解析出来，校验才是真校验：配置的合法性本就是相对世界而言
		// 的（彩装 #123 合不合法，取决于你有没有这件），故这步必须在 Validate 之前、且在 gc 已备好
		// 之后——这也正是它在 runOne 而不在 Params() 里的原因。重跑时重解析一遍：世界已经变了。
		params, err := resolveParams(ctx, gc, m)
		if err != nil {
			return Result{Meta: m.Meta(), Status: StatusError, Log: rep.lines,
				Err: fmt.Errorf("解析参数候选: %w", err)}
		}
		if err := Validate(params, values); err != nil {
			return Result{Meta: m.Meta(), Status: StatusError, Log: rep.lines,
				Err: fmt.Errorf("配置无效: %w", err)}
		}
		rc := &RunContext{Config: resolve(params, values), rep: rep, collector: col}
		err = m.Run(ctx, gc, rc)

		// 会话断点 + 模块声明可重跑 → 从头再来一次（只一次：再断多半是持续被顶号，
		// 继续重跑只会没完没了地重复副作用）。
		if _, broke := gameerr.AsSessionBreak(err); broke && policy == BreakRestart && attempt == 0 {
			rep.Logf("会话在执行期间失效，已重新登录；本模块声明可重跑，从头重试一次")
			continue
		}

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
}

func isSkip(err error) bool {
	var se *skipError
	return errors.As(err, &se)
}
