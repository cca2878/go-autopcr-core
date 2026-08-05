package app

import (
	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 门面的'错误公开面'。core 之下全是 internal，外壳 import 不到那些包——不在这里提升，
// 外壳就只能拿 err.Error() 的中文串做字符串匹配，那等于没有错误类型。
//
// 提升什么按一条线取舍：外壳'拿它能做出不同动作'的才提升。故这里是两层——
//   - ErrorClass（Domain×Kind）+ ClassifyError：定位，回答"是哪一部分出的问题"与"该怎么
//     办"，一个 switch 覆盖所有错误，新增的错误类型也自动落位；
//   - 少数几个具体类型/哨兵：细节，回答"服务器给的 result_code 是多少""是不是维护中"
//     这类只有具体类型才答得了的问题。
//
// 反过来，只是"类型不同但处置相同"的就不提升（如网络错误与协议错误，外壳看
// ErrorTransient / ErrorCorrupt 已经够了）——公开面越小越好维护。

// ErrorClass 是错误的完整定位：来源域 + 处置类别。
type ErrorClass = errs.Class

// ErrorDomain 是错误的来源子系统——回答"是哪一部分出的问题"。
type ErrorDomain = errs.Domain

// 来源域取值。
//
// 少了这一维，处置类别不足以让外壳行动：同样是 ErrorCorrupt，来自 ErrorFromGameAPI 意味着
// 响应解不开（多半客户端版本对不上，该提示更新），来自 ErrorFromMasterdata 则是资源包下坏
// 了（清掉重下即可）——两者处置相反。ErrorEnvironment、ErrorTransient 同理。
const (
	ErrorFromUnknown    = errs.DomainUnknown    // 未自报来源（含第三方库直接冒上来的）
	ErrorFromGameAPI    = errs.DomainGameAPI    // 与游戏服务器交互：链路、响应、业务码、风控、会话
	ErrorFromMasterdata = errs.DomainMasterdata // 母数据链：CDN、解包、反混淆、落盘、本地库
	ErrorFromCredential = errs.DomainCredential // 凭据侧：渠道与四要素、登录、验证码
	ErrorFromAutomation = errs.DomainAutomation // 自动化侧：模块参数与候选、前置条件、调度
	ErrorFromApp        = errs.DomainApp        // 门面装配：调用顺序、会话生命周期
)

// ErrorKind 是错误的处置类别——回答"该怎么办"。
type ErrorKind = errs.Kind

// 处置类别取值。
const (
	ErrorUnknown     = errs.KindUnknown     // 未分类：按最保守方式对待，不要自动重试
	ErrorTransient   = errs.KindTransient   // 这次不行下次可能行：重试或换个来源
	ErrorRejected    = errs.KindRejected    // 对端明确回绝：把理由呈现给用户，不要重试
	ErrorCorrupt     = errs.KindCorrupt     // 拿到的数据形状不对：丢掉重取
	ErrorMisuse      = errs.KindMisuse      // 输入/装配错了：让用户改，或外壳自己修正
	ErrorInternal    = errs.KindInternal    // 我们的缺陷：请报告，用户改什么都没用
	ErrorUnsupported = errs.KindUnsupported // 已知边界，本实现没覆盖这个情形
	ErrorEnvironment = errs.KindEnvironment // 本地环境：磁盘、权限、文件打不开
)

// ClassifyError 定位一个来自本库的错误（沿错误链取最外层自报的归类）。
//
// 这是外壳处理错误的'默认入口'：按 .Domain 决定"是谁的事"、按 .Kind 决定"怎么办"，
// 需要细节时再 errors.As 下面那几个具体类型。非本库产生的错误两维都是 Unknown。
func ClassifyError(err error) ErrorClass { return errs.Classify(err) }

// —— 具体错误类型（仅提升外壳确有动作可做的几个）——
type (
	// APIError 是游戏服务器返回的业务错误，带 result_code / status / 原始消息。
	APIError = gameerr.APIError
	// PanicError 表示应中止整条流程的致命态，最常见的来源是服务器维护中。它与普通
	// APIError 同属 ErrorRejected，但外壳往往要为它换一套提示，故单独提升。
	PanicError = gameerr.PanicError
	// RiskError 表示账号触发风控(is_risk)且未能解除，Payload 里带服务器响应的未建模字段。
	RiskError = gameerr.RiskError
	// SessionBreakError 表示会话在任务执行期间失效、客户端已重登，但这次任务跨越了世界
	// 断点、结果不可信。外壳据此提示"复查后重跑"。
	SessionBreakError = gameerr.SessionBreakError
)

// —— 哨兵 ——
var (
	// ErrNotLoggedIn 表示在未成功 Login 的会话上调用了需要登录的方法。
	ErrNotLoggedIn = errs.DomainApp.New(errs.KindMisuse, "会话未登录：请先 Login")

	// ErrNoSolver 是"未注入验证码求解器"哨兵（转发 captcha.ErrNoSolver）：触发风控而无
	// 求解器时，登录以它硬失败。外壳可 errors.Is 命中它，据此提示用户或注入求解器。
	ErrNoSolver = captcha.ErrNoSolver

	// ErrMasterdataUnavailable 表示模块要母数据、但这次运行没有启用它。外壳据此可以带上
	// 母数据重跑一次（Login 的 withMasterdata 参数）。
	ErrMasterdataUnavailable = automation.ErrMasterdataUnavailable
)
