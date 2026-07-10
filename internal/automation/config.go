package automation

import (
	"fmt"
	"slices"
)

// ParamType 是模块配置参数的类型。
type ParamType string

const (
	ParamBool   ParamType = "bool"
	ParamInt    ParamType = "int"
	ParamString ParamType = "string"
	ParamChoice ParamType = "choice" // 从 Bounds.Choices 单选（值为 string）
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
	pm := make(map[string]Param, len(params))
	for _, p := range params {
		pm[p.Name] = p
	}
	for name, v := range provided {
		p, ok := pm[name]
		if !ok {
			return fmt.Errorf("未知参数 %q", name)
		}
		if err := p.validate(v); err != nil {
			return fmt.Errorf("参数 %q: %w", name, err)
		}
	}
	return nil
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
