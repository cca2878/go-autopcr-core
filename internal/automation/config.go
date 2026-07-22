package automation

import (
	"fmt"
	"slices"
)

// ParamType 是模块配置参数的类型。
type ParamType string

const (
	ParamBool        ParamType = "bool"
	ParamInt         ParamType = "int"
	ParamString      ParamType = "string"
	ParamChoice      ParamType = "choice"      // 从 Bounds.Choices 单选（值为 string）
	ParamMultiChoice ParamType = "multichoice" // 从 Bounds.Choices 多选（值为【有序】[]string；顺序有意义时即优先级）
)

// Bounds 是参数的通用约束/边界，各类型按需使用（零值=不约束）：ParamInt 用 Min/Max，
// ParamChoice / 受限 ParamString 用 Choices。后续新增约束（如字符串长度、正则）在此扩展。
type Bounds struct {
	Min, Max *int     // 数值下/上界（含），nil=不限
	Choices  []string // 允许取值集合，nil=不限
}

// Param 是模块的参数【定义】——随模块（Params()）走，不随实际值重复携带。
type Param struct {
	Name        string
	Type        ParamType
	Default     any
	Description string
	Bounds      Bounds
}

// Option 是一个候选项：写回配置的【值】与给人看的【显示名】分开。依赖世界的候选里裸值常常
// 对人毫无意义（彩装是 serial_id、公会是 guild_id），而怎么把它显示成人话是【模块知识】，
// 故与候选解析写在一处（见 Candidates）。校验只认 Value。
type Option struct {
	Value string
	Label string
}

// isChoice 报告某参数类型的取值是否受候选约束——这类参数没有候选就是无意义的（不约束的
// Choice 即 String），故「Choice 且无候选」不是一种合法声明，见 bindCandidates。
func isChoice(t ParamType) bool { return t == ParamChoice || t == ParamMultiChoice }

// hasUnboundChoice 报告 params 里是否有【无静态候选】的 Choice 类参数——即其候选只能依赖
// 世界解析，模块因此必须实现 Candidates。供 Registry.Register 在注册期核对。
func hasUnboundChoice(params []Param) (Param, bool) {
	for _, p := range params {
		if isChoice(p.Type) && len(p.Bounds.Choices) == 0 {
			return p, true
		}
	}
	return Param{}, false
}

// bindCandidates 把解析出的候选填进对应参数的 Bounds.Choices，产出【已绑定】的参数定义（不改
// 入参）。Bounds.Choices 始终是唯一的候选源——依赖世界的参数只是要到 gc 可用时才填得上。
//
// 两处防御都【响亮失败】，因为二者都会让参数静默退回零校验，而零校验正是本机制要消灭的：
//   - 候选给了未声明的参数：多半是参数名拼错，静默则该参数永远拿不到候选；
//   - Choice 类参数既无静态候选、Candidates 也没给：Bounds.Choices 空＝不约束，静默则可传任意值。
//
// 【给了空候选】不在此列：它表示「世界里当前没有可选项」（如新号一件彩装都没有），是合法
// 状态，此时无可校验，由模块自己的 Skip 守卫接管。故这里以 map 的 key 是否存在区分「给了但
// 是空的」与「压根没给」。
func bindCandidates(params []Param, cands map[string][]Option) ([]Param, error) {
	declared := make(map[string]bool, len(params))
	for _, p := range params {
		declared[p.Name] = true
	}
	for name := range cands {
		if !declared[name] {
			return nil, fmt.Errorf("候选给了未声明的参数 %q", name)
		}
	}

	out := make([]Param, len(params))
	copy(out, params)
	for i := range out {
		if opts, given := cands[out[i].Name]; given {
			vals := make([]string, len(opts))
			for j, o := range opts {
				vals[j] = o.Value
			}
			out[i].Bounds.Choices = vals
			continue
		}
		if isChoice(out[i].Type) && len(out[i].Bounds.Choices) == 0 {
			return nil, fmt.Errorf("参数 %q 声明为 %s，却既无静态候选、Candidates 也未给出", out[i].Name, out[i].Type)
		}
	}
	return out, nil
}

// validate 校验单个值是否满足该参数的类型与边界。
func (p Param) validate(v any) error {
	switch p.Type {
	case ParamBool:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("应为 bool")
		}
	case ParamInt:
		n, ok := asInt(v)
		if !ok {
			return fmt.Errorf("应为整数")
		}
		if p.Bounds.Min != nil && n < *p.Bounds.Min {
			return fmt.Errorf("不得小于 %d", *p.Bounds.Min)
		}
		if p.Bounds.Max != nil && n > *p.Bounds.Max {
			return fmt.Errorf("不得大于 %d", *p.Bounds.Max)
		}
	case ParamString, ParamChoice:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("应为字符串")
		}
		if len(p.Bounds.Choices) > 0 && !slices.Contains(p.Bounds.Choices, s) {
			return fmt.Errorf("应为 %v 之一", p.Bounds.Choices)
		}
	case ParamMultiChoice:
		// 与其余类型一致地拒绝 null：resolve 把 null 当「未提供」回落默认值，若此处放行，
		// 用户清空的多选会被静默改回默认选项。要表达空选择请传 []。
		if v == nil {
			return fmt.Errorf("应为字符串数组（清空请传 []）")
		}
		ss, ok := asStringSlice(v)
		if !ok {
			return fmt.Errorf("应为字符串数组")
		}
		if len(p.Bounds.Choices) > 0 {
			for _, s := range ss {
				if !slices.Contains(p.Bounds.Choices, s) {
					return fmt.Errorf("%q 不在允许取值 %v 内", s, p.Bounds.Choices)
				}
			}
		}
	}
	return nil
}

// Source 是「模块名 → 参数名 → 值」的配置源（CLI 常见用法：每模块一份；同模块多实例请直接
// 构造 []Task）。
type Source map[string]map[string]any

// Config 是单个任务解析后的【只读有效值】：模块参数名→有效值（默认已合并）。参数定义（Param）
// 不在此重复——它属于模块。
type Config map[string]any

// Bool 取布尔参数（缺失/类型不符返回 false）。
func (c Config) Bool(name string) bool { b, _ := c[name].(bool); return b }

// Int 取整数参数（兼容 JSON 数字的 float64；缺失/类型不符返回 0）。
func (c Config) Int(name string) int { n, _ := asInt(c[name]); return n }

// String 取字符串参数（缺失/类型不符返回 ""）。
func (c Config) String(name string) string { s, _ := c[name].(string); return s }

// Strings 取多选参数的【有序】字符串切片（缺失/类型不符返回 nil；顺序即用户所选顺序）。
func (c Config) Strings(name string) []string { ss, _ := asStringSlice(c[name]); return ss }

// resolve 用参数定义把 provided 补全默认，产出只含【已声明参数】的有效值（不改 provided）。
func resolve(params []Param, provided map[string]any) Config {
	out := make(Config, len(params))
	for _, p := range params {
		if v, ok := provided[p.Name]; ok && v != nil {
			out[p.Name] = v
		} else {
			out[p.Name] = p.Default
		}
	}
	return out
}

// Validate 校验 provided 的每个值：参数须已声明、类型匹配、且满足 Bounds。provided 为空即通过。
func Validate(params []Param, provided map[string]any) error {
	if len(provided) == 0 {
		return nil
	}
	// 按【参数声明序】而非 map 迭代序校验：否则同一份非法配置每次跑出来的报错都可能不同，
	// 破坏「同输入同输出」的确定性契约（也让外壳侧的golden 比对失效）。
	declared := make(map[string]struct{}, len(params))
	for _, p := range params {
		declared[p.Name] = struct{}{}
		v, ok := provided[p.Name]
		if !ok {
			continue
		}
		if err := p.validate(v); err != nil {
			return fmt.Errorf("参数 %q: %w", p.Name, err)
		}
	}
	unknown := make([]string, 0, len(provided))
	for name := range provided {
		if _, ok := declared[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown) // 同理：未知参数也按名排序，报错稳定
		return fmt.Errorf("未知参数 %q", unknown[0])
	}
	return nil
}

// asStringSlice 把配置值规整为 []string：兼容 []string 与 JSON 解出的 []any（元素须为 string）。
// nil 视为空选择（合法）。任一元素非字符串则失败。
func asStringSlice(v any) ([]string, bool) {
	switch s := v.(type) {
	case nil:
		return nil, true
	case []string:
		return s, true
	case []any:
		out := make([]string, len(s))
		for i, e := range s {
			str, ok := e.(string)
			if !ok {
				return nil, false
			}
			out[i] = str
		}
		return out, true
	}
	return nil, false
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}
