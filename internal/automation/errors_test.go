package automation

import (
	"errors"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

func intp(n int) *int { return &n }

// 校验失败要答两个问题：【哪个参数】和【什么毛病】。前者靠 ParamError.Param，后者靠原因哨兵。
// 只给一句拼好的中文串，外壳就没法把错误落到表单的某一栏上。
func TestValidateReportsParamAndReason(t *testing.T) {
	params := []Param{
		{Name: "compact", Type: ParamBool},
		{Name: "count", Type: ParamInt, Bounds: Bounds{Min: intp(1), Max: intp(10)}},
		{Name: "guild_id", Type: ParamChoice, Bounds: Bounds{Choices: []string{"1", "2"}}},
		{Name: "bosses", Type: ParamMultiChoice, Bounds: Bounds{Choices: []string{"a", "b"}}},
	}

	cases := []struct {
		name      string
		provided  map[string]any
		wantParam string
		wantCause error
	}{
		{"布尔给了字符串", map[string]any{"compact": "yes"}, "compact", ErrParamType},
		{"整数给了字符串", map[string]any{"count": "3"}, "count", ErrParamType},
		{"低于下界", map[string]any{"count": 0}, "count", ErrParamRange},
		{"高于上界", map[string]any{"count": 11}, "count", ErrParamRange},
		{"选项不在候选内", map[string]any{"guild_id": "999"}, "guild_id", ErrParamChoice},
		{"多选含候选外的值", map[string]any{"bosses": []string{"a", "z"}}, "bosses", ErrParamChoice},
		{"参数名拼错", map[string]any{"compcat": true}, "compcat", ErrUnknownParam},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(params, c.provided)
			if err == nil {
				t.Fatal("应校验失败")
			}
			var pe *ParamError
			if !errors.As(err, &pe) {
				t.Fatalf("应为 *ParamError，得到 %T: %v", err, err)
			}
			if pe.Param != c.wantParam {
				t.Errorf("Param = %q, want %q", pe.Param, c.wantParam)
			}
			if !errors.Is(err, c.wantCause) {
				t.Errorf("原因应命中 %v，得到 %v", c.wantCause, err)
			}
			// 用户配置错了，不是程序坏了。
			if got := errs.Classify(err).Kind; got != errs.KindMisuse {
				t.Errorf("Classify = %v, want KindMisuse", got)
			}
		})
	}
}

func TestValidateAcceptsGoodValues(t *testing.T) {
	params := []Param{
		{Name: "count", Type: ParamInt, Bounds: Bounds{Min: intp(1), Max: intp(10)}},
		{Name: "bosses", Type: ParamMultiChoice, Bounds: Bounds{Choices: []string{"a", "b"}}},
	}
	if err := Validate(params, map[string]any{"count": 5, "bosses": []string{"b", "a"}}); err != nil {
		t.Fatalf("合法配置不应报错：%v", err)
	}
}

// 母数据缺席是【装配错误】：各模块措辞不同，但判定必须统一，否则外壳只能去匹配中文串。
func TestRequireMasterdata(t *testing.T) {
	err := RequireMasterdata("判定赛马开放时段")

	if !errors.Is(err, ErrMasterdataUnavailable) {
		t.Errorf("应命中 ErrMasterdataUnavailable，得到 %v", err)
	}
	if got := errs.Classify(err).Kind; got != errs.KindMisuse {
		t.Errorf("Classify = %v, want KindMisuse", got)
	}
	if msg := err.Error(); msg != "母数据未启用：本模块需要它判定赛马开放时段" {
		t.Errorf("Error() = %q", msg)
	}
	// 它绝不能被当成「跳过」——那会把一次配置事故伪装成正常运行。
	if isSkip(err) {
		t.Error("母数据缺席不是 Skip")
	}
}

// 候选声明与参数声明对不上是模块作者的 bug，用户改配置没用，故与用户输入错误分属两类。
func TestBadCandidatesIsInternal(t *testing.T) {
	params := []Param{{Name: "guild_id", Type: ParamChoice}}

	_, err := bindCandidates(params, map[string][]Option{"guild_di": {{Value: "1"}}})
	if !errors.Is(err, ErrBadCandidates) {
		t.Fatalf("拼错的候选参数名应命中 ErrBadCandidates，得到 %v", err)
	}
	if got := errs.Classify(err).Kind; got != errs.KindInternal {
		t.Errorf("Classify = %v, want KindInternal", got)
	}

	// Choice 类参数既无静态候选、Candidates 也没给出，同样是装配问题。
	_, err = bindCandidates(params, nil)
	if !errors.Is(err, ErrBadCandidates) {
		t.Fatalf("无候选的 Choice 参数应命中 ErrBadCandidates，得到 %v", err)
	}
	var pe *ParamError
	if !errors.As(err, &pe) || pe.Param != "guild_id" {
		t.Errorf("应指出是哪个参数，得到 %v", err)
	}
}
