package app

import (
	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client/credential/accesskey"
	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
)

// 以下别名把 app 之下各内部域的类型提升为 app 包的'公开命名面'：外部消费方（如
// go-autopcr-mobile）无需 import 内部包即可命名它们，且零转换成本（别名即同一类型）。
// 这是 app 作为'三前端共享的应用服务门面'应有之义——门面之下才是 internal。

// —— 自动化域：模块 / 任务 / 结果 ——
type (
	Registry    = automation.Registry    // 模块注册表（列出/挑选模块与预设）
	Module      = automation.Module      // 单个自动化任务单元
	Meta        = automation.Meta        // 模块静态元信息
	Param       = automation.Param       // 模块参数定义
	ParamType   = automation.ParamType   // 参数类型（Param.Type 的类型）
	Bounds      = automation.Bounds      // 参数约束/边界
	Preset      = automation.Preset      // 具名模块批
	Task        = automation.Task        // 一次待执行任务的纯数据描述（可序列化）
	Result      = automation.Result      // 单个任务运行结果
	Source      = automation.Source      // "模块名→参数名→值"配置源
	Status      = automation.Status      // 任务结果状态
	Observer    = automation.Observer    // 只写进度端口（外壳注入、核心推送）
	Event       = automation.Event       // 任务级进度事件
	Phase       = automation.Phase       // 进度事件阶段
	Collector   = automation.Collector   // 只写遥测端口（外壳注入、模块 Emit 推送）
	Observation = automation.Observation // 一条结构化遥测观测（Kind + Fields）
)

// 参数类型常量（转发 automation 同名量）。与 ParamType 一同导出是必须的：只导出 Param
// 而不导出其 Type 字段的类型与取值，外部就无法对参数类型做分支——数据驱动表单（按参数
// 类型渲染控件）正是门面消费方的典型用法。
const (
	ParamBool        = automation.ParamBool        // 布尔
	ParamInt         = automation.ParamInt         // 整数（Bounds.Min/Max）
	ParamString      = automation.ParamString      // 字符串
	ParamChoice      = automation.ParamChoice      // 从 Bounds.Choices 单选
	ParamMultiChoice = automation.ParamMultiChoice // 从 Bounds.Choices 多选（值为'有序'[]string）
)

// 进度事件阶段常量（转发 automation 同名量）。
const (
	PhaseStarted  = automation.PhaseStarted
	PhaseFinished = automation.PhaseFinished
)

// 任务结果状态常量（转发 automation 同名量）。
const (
	StatusOK    = automation.StatusOK
	StatusSkip  = automation.StatusSkip
	StatusError = automation.StatusError
)

// 渠道标识（转发 accesskey 同名量）：登录 / 母数据刷新的 channel 取此。
const (
	ChannelBSDK = accesskey.ChannelBSDK // 官服
	ChannelQSDK = accesskey.ChannelQSDK // 渠道服
)

// TasksFor 为选中的模块各配一份来自 src 的配置，产出 []Task（转发 automation.TasksFor）。
func TasksFor(mods []Module, src Source) []Task { return automation.TasksFor(mods, src) }

// —— 玩家状态 / 母数据只读面 / 验证码端口 ——
//
// CaptchaResult 与 Solver 一同导出是必须的：Solver 的方法签名引用它，只导出接口而不导出其
// 结果类型，模块外就'实现不了'这个端口。名字没跟着 captcha.Result 走，是因为 Result 在本
// 门面已归任务运行结果所有（见上）；求解结果是另一回事，故显式冠以 Captcha。
type (
	PlayerState   = gamestate.PlayerState // 聚合玩家状态
	Reader        = masterdata.Reader     // 母数据只读查询面
	Solver        = captcha.Solver        // 验证码求解端口（外壳注入其实现）
	CaptchaResult = captcha.Result        // 一次求解的结果（Solver 的返回）
)

// 错误的公开面（哨兵、可 errors.As 的类型、处置类别）见 errors.go。

// MasterdataHandle 是须显式 Close 的母数据只读句柄（RefreshMasterdata 的返回类型——
// 免登录刷新拿到的库由调用方持有并关闭）。
type MasterdataHandle interface {
	Reader
	Close() error
}
