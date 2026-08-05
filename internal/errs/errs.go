// Package errs 给全仓的错误定一个'顶层归类'。
//
// 归类有两个维度，缺一个都不足以处理错误：
//
//   - Domain 回答'是哪一部分出的问题'——母数据链、游戏服务器、凭据、自动化、门面；
//   - Kind 回答'该怎么办'——重试、让用户改输入、清掉重取、报缺陷……
//
// 为什么必须是两维：单给 Kind，"数据损坏" 既可能是游戏响应解不开（多半是客户端版本对不上，
// 该提示更新），也可能是资源包下坏了（清掉重下即可）——处置完全相反，而调用方分不出来。
// 单给 Domain 则只知道谁出了事、不知道能不能重试。两者叉乘才构成一个可处理的定位。
//
// 各域自然会长出自己的错误类型，它们回答的是更细的"具体是什么"（哪个参数、哪个版本、
// 哪一步、哪个 URL）。本包不取代它们：Domain×Kind 是默认入口，errors.As 到具体类型是深入
// 一层的手段。少了前者，每个消费者都得自攒一张"哪些算可重试"的清单，且新增错误类型时都要
// 记得同步更新——分类逻辑散在各处、又缺一个统一入口强制归位，就等于没有分类。
package errs

import "errors"

// Domain 是错误的来源子系统——按'调用方的处置动作是否不同'划分，不是按包划分。
// 清缓存重下、重新登录、改配置、稍后再试，这几件事各自对应一个 Domain。
type Domain int

const (
	// DomainUnknown 是未自报来源的错误（含第三方库直接冒上来的）。
	DomainUnknown Domain = iota

	// DomainGameAPI 是与游戏服务器交互产生的：链路、响应解析、业务错误码、风控、会话。
	DomainGameAPI

	// DomainMasterdata 是母数据链上的：CDN 清单与下载、资源包解包、反混淆、落盘、本地库打开。
	DomainMasterdata

	// DomainCredential 是凭据侧的：渠道与四要素、登录、验证码求解器。
	DomainCredential

	// DomainAutomation 是自动化侧的：模块参数与候选、模块前置条件、任务调度。
	DomainAutomation

	// DomainApp 是门面装配侧的：调用顺序、会话生命周期。
	DomainApp
)

func (d Domain) String() string {
	switch d {
	case DomainGameAPI:
		return "游戏服务"
	case DomainMasterdata:
		return "母数据"
	case DomainCredential:
		return "凭据"
	case DomainAutomation:
		return "自动化"
	case DomainApp:
		return "门面"
	default:
		return "未知来源"
	}
}

// Kind 是错误的处置类别。取值刻意少而正交——每一个都对应一种明确不同的应对方式，
// 分不出应对方式差别的就不该是新类别。
type Kind int

const (
	// KindUnknown 是未自报类别的错误。消费者应按最保守的方式对待：不自动重试、原样呈现。
	KindUnknown Kind = iota

	// KindTransient 表示这次不行、下次可能就行：网络抖动、CDN 5xx、会话被顶掉后已重登。
	// 应对＝重试，或换一个来源再来一次。
	KindTransient

	// KindRejected 表示对端明确回绝了，重试多少次都是同一个结果：游戏服务器的业务错误码、
	// 资源 404、风控没过。应对＝把对端给的理由呈现给用户。
	KindRejected

	// KindCorrupt 表示拿到的数据不是它该有的形状。应对＝丢掉这份数据重取；反复如此则多半
	// 是协议或版本对不上了。具体怎么"重取"取决于 Domain——母数据是清缓存重下，游戏服务
	// 是重发请求。
	KindCorrupt

	// KindMisuse 表示调用方给错了东西：渠道名不对、uid 为空、模块参数越界、母数据没启用。
	// 应对＝让用户改输入，或让外壳修正装配。程序本身没坏。
	KindMisuse

	// KindInternal 表示我们自己的代码有 bug。应对＝报告缺陷，用户侧改什么都没用。
	KindInternal

	// KindUnsupported 表示遇到了本实现没有覆盖的情形（如 LZMA 压缩的资源包）。它不是 bug
	// 而是已知边界，应对＝告知用户这个场景暂不支持。
	KindUnsupported

	// KindEnvironment 表示本地环境的问题：磁盘写不进、目录没权限、库文件打不开。
	// 应对＝引导用户检查存储与权限。
	KindEnvironment
)

func (k Kind) String() string {
	switch k {
	case KindTransient:
		return "暂时性故障"
	case KindRejected:
		return "对端拒绝"
	case KindCorrupt:
		return "数据损坏"
	case KindMisuse:
		return "用法错误"
	case KindInternal:
		return "内部缺陷"
	case KindUnsupported:
		return "暂不支持"
	case KindEnvironment:
		return "本地环境"
	default:
		return "未分类"
	}
}

// Class 是一个错误的完整归类：来自哪一部分 + 该怎么办。
type Class struct {
	Domain Domain
	Kind   Kind
}

func (c Class) String() string { return c.Domain.String() + "·" + c.Kind.String() }

// With 把本域与一个处置类别组合成 Class：errs.DomainMasterdata.With(errs.KindCorrupt)。
func (d Domain) With(k Kind) Class { return Class{Domain: d, Kind: k} }

// New 声明一个属于本域的哨兵错误：errs.DomainMasterdata.New(errs.KindInternal, "…")。
// 以域为接收者，是因为同一个包里的哨兵域都相同，这样写不必每条都重复它。
func (d Domain) New(kind Kind, msg string) *Sentinel {
	return &Sentinel{msg: msg, class: d.With(kind)}
}

// classifier 由自报归类的错误实现。方法名没取 Class()，是为了不和各处已有的同名字段撞。
type classifier interface{ ErrorClass() Class }

// Classify 返回错误链上'最外层'自报的归类；无人自报则两维都是 Unknown。
//
// 取最外层是有意的：包装它的那一层，往往正是对"这个失败是谁的事、该怎么处理"做了判断的
// 一层。传输层把解码失败包进 NetworkError 就是这样一次判断——它明知里面是 ProtocolError，
// 仍决定按可重试对待。想看穿这层判断、拿到底下的具体成因，用 errors.As 点名要即可。
func Classify(err error) Class {
	var c classifier
	if errors.As(err, &c) {
		return c.ErrorClass()
	}
	return Class{}
}

// Is 报告错误的处置类别是否为 k（不问来源）。
func Is(err error, k Kind) bool { return Classify(err).Kind == k }

// From 报告错误是否来自域 d（不问处置类别）。
func From(err error, d Domain) bool { return Classify(err).Domain == d }

// Sentinel 是自带归类的哨兵错误：既能被 errors.Is 精确命中，又能被 Classify 归类。
//
// 各域的"就这一种情况、没有额外字段要带"的错误用它声明；需要携带上下文（哪个参数、
// 哪个版本、哪个 URL）的，请定义自己的结构体类型并实现 ErrorClass。
type Sentinel struct {
	msg   string
	class Class
}

func (e *Sentinel) Error() string { return e.msg }

// ErrorClass 实现归类。
func (e *Sentinel) ErrorClass() Class { return e.class }
