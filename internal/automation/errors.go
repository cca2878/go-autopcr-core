package automation

import (
	"fmt"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// ErrMasterdataUnavailable 表示模块运行时拿不到母数据只读句柄（gc.Masterdata() 为 nil）。
//
// 这是【装配错误】，不是模块的业务判定：要么上层没为这批模块启用 WithMasterdata，要么母
// 数据构建本身失败了。故模块遇到它一律返回错误、让任务落到 StatusError，绝不能当成「今天
// 没事可做」静默 Skip——那会把一次配置事故伪装成正常运行。
//
// 各模块用 RequireMasterdata 构造，措辞随模块，判定靠这个哨兵。
var ErrMasterdataUnavailable = errs.DomainAutomation.New(errs.KindMisuse, "母数据未启用")

// RequireMasterdata 构造一个「本模块要母数据，但它不在」的错误。purpose 是动宾短语，说明
// 这个模块拿母数据做什么（如 "判定赛马开放时段"），用于让用户看懂到底缺了什么。
func RequireMasterdata(purpose string) error {
	return fmt.Errorf("%w：本模块需要它%s", ErrMasterdataUnavailable, purpose)
}

// 参数校验失败的三种原因。分开命名，是为了让数据驱动的配置界面能按原因给出不同的引导——
// 类型不符该换控件，越界该提示区间，不在候选内则该重新拉一次候选（它依赖账号/母数据，可能
// 只是过期了）。
var (
	ErrParamType   = errs.DomainAutomation.New(errs.KindMisuse, "类型不符")
	ErrParamRange  = errs.DomainAutomation.New(errs.KindMisuse, "超出取值范围")
	ErrParamChoice = errs.DomainAutomation.New(errs.KindMisuse, "不在候选取值内")

	// ErrUnknownParam 表示配置里给了模块没有声明的参数（多半是名字拼错）。
	ErrUnknownParam = errs.DomainAutomation.New(errs.KindMisuse, "未知参数")

	// ErrUnknownModule 表示 Task.Module 这个名字在 Registry 里找不到对应模块。批处理中它
	// 只让该任务失败、不影响其余任务，故外壳想把它与真正的运行失败区分开时用得上。
	ErrUnknownModule = errs.DomainAutomation.New(errs.KindMisuse, "未知模块")

	// ErrBadCandidates 表示模块的 Candidates 实现与它自己的 Params 声明对不上。这是
	// 【模块作者的装配错误】而非用户配置问题——用户改什么都没用，故归 KindInternal 而非
	// 与上面几个同类。
	ErrBadCandidates = errs.DomainAutomation.New(errs.KindInternal, "参数候选声明有误")
)

// ParamError 指出是【哪个】参数出了问题，Err 是上面几个原因之一。
//
// 带上参数名，是为了让外壳把错误落到配置表单的具体那一栏，而不是把一整句中文丢给用户去猜。
type ParamError struct {
	Param string
	Err   error
}

func (e *ParamError) Error() string { return fmt.Sprintf("参数 %q: %v", e.Param, e.Err) }

func (e *ParamError) Unwrap() error { return e.Err }
