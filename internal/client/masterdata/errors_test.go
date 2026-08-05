package masterdata

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/unityfs"
	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 构建链上的每一步都带着版本号与阶段冒上来——过去落盘失败只给一句裸的 "permission denied"，
// 既不知道是哪个版本，也不知道停在哪一步。
func TestBuildErrorCarriesVerAndStage(t *testing.T) {
	err := buildErr(120250, StageDownload, errors.New("连不上"))
	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("应为 *BuildError，得到 %T", err)
	}
	if be.Ver != 120250 || be.Stage != StageDownload {
		t.Errorf("Ver/Stage = %d/%q", be.Ver, be.Stage)
	}
	if msg := err.Error(); msg != "构建母数据库 v120250 失败于「下载资源包」: 连不上" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestBuildErrNilPassesThrough(t *testing.T) {
	if err := buildErr(1, StageStore, nil); err != nil {
		t.Errorf("成因为 nil 时应返回 nil，得到 %v", err)
	}
}

// 类别'委托给成因'：下载/解包那几步究竟属哪一类，只有里面那层知道，BuildError 不替它拍板。
func TestBuildErrorDelegatesKindToCause(t *testing.T) {
	cases := []struct {
		name  string
		stage BuildStage
		cause error
		want  errs.Kind
	}{
		{"解包遇到损坏数据", StageExtract, unityfs.ErrMalformed, errs.KindCorrupt},
		{"解包遇到不支持的特性", StageExtract, unityfs.ErrUnsupported, errs.KindUnsupported},
		{"下载被对端拒绝", StageDownload, errs.DomainMasterdata.New(errs.KindRejected, "404"), errs.KindRejected},
		{"下载遇到暂时故障", StageDownload, errs.DomainMasterdata.New(errs.KindTransient, "503"), errs.KindTransient},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errs.Classify(buildErr(1, c.stage, c.cause)).Kind; got != c.want {
				t.Errorf("Classify = %v, want %v", got, c.want)
			}
			if !errors.Is(buildErr(1, c.stage, c.cause), c.cause) {
				t.Error("包装后应仍能 errors.Is 命中成因")
			}
		})
	}
}

// 落盘与反混淆包的是 os / database/sql 的裸错误，它们不可能自报类别；没有按阶段的兜底，
// 一次"磁盘写满"就会以 KindUnknown 冒到外壳，什么引导都给不出。
func TestBuildErrorFallsBackToStageForSilentCauses(t *testing.T) {
	if got := errs.Classify(fs.ErrPermission).Kind; got != errs.KindUnknown {
		t.Fatalf("前提失效：os 裸错误本应无类别，得到 %v", got)
	}

	for _, stage := range []BuildStage{StageStore, StageUnhash} {
		if got := errs.Classify(buildErr(1, stage, fs.ErrPermission)).Kind; got != errs.KindEnvironment {
			t.Errorf("阶段 %q：Classify = %v, want KindEnvironment", stage, got)
		}
	}
	// 下载/解包没有本地兜底——那两步的成因本就该自报，兜底反而会掩盖分类缺失。
	if got := errs.Classify(buildErr(1, StageDownload, fs.ErrPermission)).Kind; got != errs.KindUnknown {
		t.Errorf("下载阶段的无类别成因应保持 KindUnknown，得到 %v", got)
	}
}

// 成因自报时以成因为准，阶段兜底不得反过来盖掉它。
func TestBuildErrorCauseWinsOverStageFallback(t *testing.T) {
	err := buildErr(1, StageUnhash, errs.DomainMasterdata.New(errs.KindCorrupt, "库内容不对"))
	if got := errs.Classify(err).Kind; got != errs.KindCorrupt {
		t.Errorf("Classify = %v, want KindCorrupt（成因优先于阶段兜底）", got)
	}
}

// BuildError 只把'处置类别'委托给成因，来源域始终是母数据——委托时把 Domain 一起交出去
// 就错了：成因可能是 unityfs 的（同属母数据链，看不出问题），但调用方要的是"这是母数据链
// 上的事"这个稳定答案，而不是随成因所在的包漂移。
func TestBuildErrorKeepsMasterdataDomainWhileDelegatingKind(t *testing.T) {
	cases := []error{
		unityfs.ErrMalformed,
		unityfs.ErrUnsupported,
		fs.ErrPermission, // 无类别的裸错误
		errs.DomainMasterdata.New(errs.KindTransient, "503"),
		// '跨域成因'——现实中构建链的成因都在母数据域内，正因如此，只用同域成因去测
		// 根本分不出"固定 Domain"与"连 Domain 一起委托"。这条人造用例才钉得住设计意图：
		// 将来若有别域的错误漏进构建链（如共享 transport 冒出 gameerr），调用方看到的仍
		// 应是"母数据链出事了"，而不是被带到另一个域上去。
		errs.DomainGameAPI.New(errs.KindTransient, "来自另一个域的成因"),
	}
	for _, cause := range cases {
		for _, stage := range []BuildStage{StageDownload, StageExtract, StageUnhash, StageStore} {
			if got := errs.Classify(buildErr(1, stage, cause)).Domain; got != errs.DomainMasterdata {
				t.Errorf("成因 %v / 阶段 %q：Domain = %v, want DomainMasterdata", cause, stage, got)
			}
		}
	}
}

func TestMasterdataSentinelDomains(t *testing.T) {
	for _, err := range []error{ErrBadManifestVer, ErrBadRainbow} {
		if got := errs.Classify(err).Domain; got != errs.DomainMasterdata {
			t.Errorf("%v 的 Domain = %v, want DomainMasterdata", err, got)
		}
	}
}

func TestMasterdataSentinelKinds(t *testing.T) {
	if got := errs.Classify(ErrBadManifestVer).Kind; got != errs.KindCorrupt {
		t.Errorf("ErrBadManifestVer 分类 = %v, want KindCorrupt", got)
	}
	// rainbow 随二进制发布、不来自外部，坏了只可能是我们自己打包错了。
	if got := errs.Classify(ErrBadRainbow).Kind; got != errs.KindInternal {
		t.Errorf("ErrBadRainbow 分类 = %v, want KindInternal", got)
	}
}
