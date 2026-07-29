package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelCarriesBothDimensions(t *testing.T) {
	sentinel := DomainMasterdata.New(KindEnvironment, "磁盘写不进")

	if !errors.Is(sentinel, sentinel) {
		t.Error("哨兵应能被 errors.Is 命中自身")
	}
	got := Classify(sentinel)
	if got.Domain != DomainMasterdata || got.Kind != KindEnvironment {
		t.Errorf("Classify = %v, want 母数据·本地环境", got)
	}
	if !Is(sentinel, KindEnvironment) {
		t.Error("Is 应按处置类别判定")
	}
	if !From(sentinel, DomainMasterdata) {
		t.Error("From 应按来源域判定")
	}
}

// 哨兵被 fmt.Errorf 包上一层细节后，三种判定都必须还在——这是各域的常规用法。
func TestWrappedSentinelKeepsIsAndClass(t *testing.T) {
	sentinel := DomainMasterdata.New(KindCorrupt, "数据损坏")
	wrapped := fmt.Errorf("%w：偏移 %d 越界", sentinel, 42)

	if !errors.Is(wrapped, sentinel) {
		t.Error("包装后应仍能 errors.Is 命中哨兵")
	}
	if got := Classify(wrapped); got != (Class{DomainMasterdata, KindCorrupt}) {
		t.Errorf("包装后 Classify = %v", got)
	}
}

// ★ 这是 Domain 这一维存在的理由：同一个 Kind 落在不同 Domain 上，处置完全不同。
// 「数据损坏」来自游戏服务是响应解不开（多半版本对不上，该提示更新），来自母数据是资源包
// 下坏了（清掉重下即可）。只给 Kind，调用方以为拿到了足够信息，其实分不出该做哪件事。
func TestSameKindDifferentDomainStaysDistinguishable(t *testing.T) {
	fromGame := DomainGameAPI.New(KindCorrupt, "响应解不开")
	fromMD := DomainMasterdata.New(KindCorrupt, "资源包坏了")

	if Classify(fromGame).Kind != Classify(fromMD).Kind {
		t.Fatal("前提失效：两者本就该是同一个 Kind")
	}
	if Classify(fromGame).Domain == Classify(fromMD).Domain {
		t.Error("同 Kind 不同来源必须能区分开，否则调用方无从决定做哪件事")
	}
	if !From(fromGame, DomainGameAPI) || !From(fromMD, DomainMasterdata) {
		t.Error("From 应分别命中各自的域")
	}
}

// 归类取【最外层】自报的，两维一起取：包装它的那一层往往正是判断了「这是谁的事、该怎么办」
// 的一层。
func TestClassifyTakesOutermost(t *testing.T) {
	inner := DomainMasterdata.New(KindCorrupt, "里层")
	outer := &classedWrapper{
		class: DomainGameAPI.With(KindTransient),
		err:   fmt.Errorf("外层: %w", inner),
	}

	if got := Classify(outer); got != (Class{DomainGameAPI, KindTransient}) {
		t.Errorf("Classify = %v, want 游戏服务·暂时性故障（最外层自报的）", got)
	}
	// 但里层仍可被点名取出——归类是默认入口，不是唯一入口。
	if !errors.Is(outer, inner) {
		t.Error("最外层归类不应挡住对内层哨兵的 errors.Is")
	}
}

// 链上没有任何一层自报时（第三方库或 os 的裸错误直接冒上来），两维都得是 Unknown：
// 消费者据此走最保守路径，而不是被误归到某个具体域或类别上。
func TestClassifyUnknownForForeignError(t *testing.T) {
	for _, err := range []error{errors.New("来自别处"), nil} {
		if got := Classify(err); got != (Class{DomainUnknown, KindUnknown}) {
			t.Errorf("Classify(%v) = %v, want 两维皆 Unknown", err, got)
		}
	}
}

func TestStrings(t *testing.T) {
	// 每个取值都得有自己的说法——落到 default 上就等于这一维没生效。
	domains := []Domain{DomainGameAPI, DomainMasterdata, DomainCredential, DomainAutomation, DomainApp}
	seenD := map[string]bool{}
	for _, d := range domains {
		s := d.String()
		if s == "未知来源" {
			t.Errorf("Domain(%d).String() 落到了 default 分支", int(d))
		}
		if seenD[s] {
			t.Errorf("Domain(%d).String() = %q 与其它域重名", int(d), s)
		}
		seenD[s] = true
	}

	kinds := []Kind{KindTransient, KindRejected, KindCorrupt, KindMisuse,
		KindInternal, KindUnsupported, KindEnvironment}
	seenK := map[string]bool{}
	for _, k := range kinds {
		s := k.String()
		if s == "未分类" {
			t.Errorf("Kind(%d).String() 落到了 default 分支", int(k))
		}
		if seenK[s] {
			t.Errorf("Kind(%d).String() = %q 与其它类别重名", int(k), s)
		}
		seenK[s] = true
	}

	if got := (Class{DomainMasterdata, KindCorrupt}).String(); got != "母数据·数据损坏" {
		t.Errorf("Class.String() = %q", got)
	}
	if got := (Class{}).String(); got != "未知来源·未分类" {
		t.Errorf("零值 Class.String() = %q", got)
	}
}

// classedWrapper 是测试用的「自报归类且可继续 Unwrap」的错误。
type classedWrapper struct {
	class Class
	err   error
}

func (e *classedWrapper) Error() string     { return e.err.Error() }
func (e *classedWrapper) Unwrap() error     { return e.err }
func (e *classedWrapper) ErrorClass() Class { return e.class }
