package masterdata

import (
	"fmt"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

var (
	// ErrBadManifestVer 表示服务端下发的 manifest_ver 不是个版本号——它是我们从游戏服务器
	// 收到的数据，形状不对属 KindCorrupt 而非用法错误。
	ErrBadManifestVer = errs.DomainMasterdata.New(errs.KindCorrupt, "manifest_ver 非法")

	// ErrBadRainbow 表示内嵌的 rainbow 反混淆表解析不了。它随二进制一起发布、不来自外部，
	// 故只可能是我们自己打包坏了：用户改什么都没用。
	ErrBadRainbow = errs.DomainMasterdata.New(errs.KindInternal, "内嵌 rainbow 表解析失败")
)

// BuildStage 标识干净母数据库构建链上的一步。
type BuildStage string

const (
	StageDownload BuildStage = "下载资源包"    // 从 CDN 取 masterdata_master.unity3d
	StageExtract  BuildStage = "提取 SQLite" // 从 UnityFS 容器里剥出库文件字节
	StageUnhash   BuildStage = "反混淆"      // 用 rainbow 表把哈希名改回真实表名/列名
	StageStore    BuildStage = "落盘缓存"     // 建目录、写临时文件、rename 到最终路径
)

// BuildError 表示某个版本的干净母数据库没能构建出来，Stage 指出它停在哪一步。
//
// 分阶段是为了让调用方不必去读错误文本就知道该怎么办：停在下载或提取，多半是网络不通或
// 这份资源包本身残缺（重来一次有戏，unityfs.ErrMalformed 那一类尤其如此）；停在反混淆
// 或落盘，则是本地环境的问题——磁盘满、目录没权限、rainbow 表与这个版本对不上，重试再多次
// 也是同一个结果。
//
// 它也补上了原先最难查的一类现场：落盘那几步（MkdirAll / CreateTemp / Rename）过去直接
// 上抛 os 的裸错误，用户只看得到一句 "permission denied"，既不知道是哪个版本、也不知道
// 是构建链上的哪一步出的事。
type BuildError struct {
	Ver   int
	Stage BuildStage
	Err   error
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("构建母数据库 v%d 失败于「%s」: %v", e.Ver, e.Stage, e.Err)
}

func (e *BuildError) Unwrap() error { return e.Err }

// ErrorClass 的来源固定是母数据（构建链本身就是这一域），处置类别则【优先听成因的】：
// 下载失败是暂时性还是被拒、解包失败是损坏还是不支持，只有里面那层知道，这层不该替它拍板。
//
// 只有当成因不吭声时（落盘与反混淆两步包的是 os 与 database/sql 的裸错误，它们不可能自报
// 类别）才按阶段兜底——那两步失败无一例外是本地这台机器的事：磁盘满、目录没权限、库文件被
// 占用或损坏。少了这个兜底，一次「磁盘写满」会以未分类冒到外壳，什么引导都给不出。
func (e *BuildError) ErrorClass() errs.Class {
	if k := errs.Classify(e.Err).Kind; k != errs.KindUnknown {
		return errs.DomainMasterdata.With(k)
	}
	switch e.Stage {
	case StageStore, StageUnhash:
		return errs.DomainMasterdata.With(errs.KindEnvironment)
	default:
		return errs.DomainMasterdata.With(errs.KindUnknown)
	}
}

// buildErr 包装构建链上某一步的失败；err 为 nil 时返回 nil，便于在调用点直接串联。
func buildErr(ver int, stage BuildStage, err error) error {
	if err == nil {
		return nil
	}
	return &BuildError{Ver: ver, Stage: stage, Err: err}
}
