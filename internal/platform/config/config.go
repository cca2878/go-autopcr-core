// Package config 加载运行期配置。
//
// 首期仅覆盖「库 + CLI」所需的最小集合：日志级别取自环境变量，
// 目录为固定值（见 paths.Default）。可随里程碑逐步扩展。
package config

import (
	"os"
	"strconv"

	"github.com/cca2878/go-autopcr-core/internal/platform/paths"
)

// Config 保存运行期配置。
type Config struct {
	// Debug 控制是否输出 debug 级别日志。
	Debug bool
	// Paths 是库所需的只读输入目录（首期为固定值）。
	Paths paths.Paths
}

// Load 读取运行期配置。
//
// 目前仅日志级别可由环境变量控制；目录为固定值（见 paths.Default）：
//
//	AUTOPCR_DEBUG  是否输出 debug 日志（默认 false）
func Load() Config {
	return Config{
		Debug: boolEnv("AUTOPCR_DEBUG", false),
		Paths: paths.Default(),
	}
}

// boolEnv 读取布尔型环境变量，缺失或无法解析时返回 def。
func boolEnv(key string, def bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return value
}
