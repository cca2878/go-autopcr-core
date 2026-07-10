// Package paths 提供首期库所需的只读输入目录配置。
//
// 首期库是纯函数式、自身不存储状态，因此只关心两个【只读输入】目录：
//
//	Cache — 预置成品的存放处（如母数据 SQLite）
//	Data  — 只读数据资产（rainbow.json、字体、extraDrops 等）
//
// 结果（result）与日志（log）分别属于上层存储与应用职责，不由本库管理。
//
// 这两个路径当前为固定值；首期完成后将改为「调用库时由调用方传入」，以适配跨平台。
package paths

// Paths 是库运行所需的只读输入目录集合。
type Paths struct {
	Cache string // 预置成品目录（如母数据 SQLite）
	Data  string // 只读数据资产目录（rainbow.json、字体等）
}

// Default 返回首期使用的固定目录配置。
//
// TODO(首期后): 改为由调用方在调用库时传入，以适配跨平台 / 自定义部署。
func Default() Paths {
	return Paths{
		Cache: "cache",
		Data:  "data",
	}
}
