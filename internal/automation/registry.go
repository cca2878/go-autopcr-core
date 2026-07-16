package automation

import "fmt"

// Preset 是一个具名的模块批（模块名列表），供「批处理预设」选择。
type Preset struct {
	Name    string
	Title   string
	Modules []string
}

// Registry 是模块注册表（名字→模块，保留注册顺序）+ 具名批预设。
type Registry struct {
	byName      map[string]Module
	order       []string
	presets     map[string]Preset
	presetOrder []string
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Module), presets: make(map[string]Preset)}
}

// Register 登记一个模块（按 Meta().Name 索引；重复名字覆盖，顺序取首次）。
//
// 声明了无静态候选的 Choice 类参数、却没实现 Candidates 的模块，在此 panic：那样的参数会静默
// 退回零校验（Bounds.Choices 空＝不约束），而这正是 Candidates 要消灭的东西。之所以 panic 而
// 不返回 error——注册表由各域 Register 在进程启动时静态装配，装配错了是程序 bug，不是运行期
// 可恢复的输入错误；而 DefaultRegistry() 是所有模块测试的必经之路，故 CI 必抓。这也是这条不变
// 量能落到的最早时机：Go 的类型系统表达不了「Choice 必有候选」。
func (r *Registry) Register(m Module) {
	name := m.Meta().Name
	if _, ok := m.(Candidates); !ok {
		if p, unbound := hasUnboundChoice(m.Params()); unbound {
			panic(fmt.Sprintf("automation: 模块 %q 的参数 %q 声明为 %s 却无候选，须实现 Candidates 以在世界已知时解析", name, p.Name, p.Type))
		}
	}
	if _, ok := r.byName[name]; !ok {
		r.order = append(r.order, name)
	}
	r.byName[name] = m
}

// Get 按名字取模块。
func (r *Registry) Get(name string) (Module, bool) {
	m, ok := r.byName[name]
	return m, ok
}

// All 按注册顺序返回全部模块。
func (r *Registry) All() []Module {
	out := make([]Module, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byName[n])
	}
	return out
}

// Select 按名字挑选模块并保持给定顺序；未识别的名字经第二返回值报告。
func (r *Registry) Select(names ...string) (mods []Module, unknown []string) {
	for _, n := range names {
		if m, ok := r.byName[n]; ok {
			mods = append(mods, m)
		} else {
			unknown = append(unknown, n)
		}
	}
	return mods, unknown
}

// ByCategory 按注册顺序返回某分类下的全部模块。
func (r *Registry) ByCategory(category string) []Module {
	var out []Module
	for _, n := range r.order {
		if m := r.byName[n]; m.Meta().Category == category {
			out = append(out, m)
		}
	}
	return out
}

// RegisterPreset 登记一个批预设（按 Name 索引，保留登记顺序）。
func (r *Registry) RegisterPreset(p Preset) {
	if _, ok := r.presets[p.Name]; !ok {
		r.presetOrder = append(r.presetOrder, p.Name)
	}
	r.presets[p.Name] = p
}

// Preset 解析批预设为模块列表；未知预设名以第二返回值 false 报告，预设内的未知模块名经第
// 三返回值报告。
func (r *Registry) Preset(name string) (mods []Module, ok bool, unknown []string) {
	p, ok := r.presets[name]
	if !ok {
		return nil, false, nil
	}
	mods, unknown = r.Select(p.Modules...)
	return mods, true, unknown
}

// Presets 按登记顺序返回全部批预设。
func (r *Registry) Presets() []Preset {
	out := make([]Preset, 0, len(r.presetOrder))
	for _, n := range r.presetOrder {
		out = append(out, r.presets[n])
	}
	return out
}
