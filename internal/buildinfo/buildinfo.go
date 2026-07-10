// Package buildinfo 暴露构建期由链接器注入的版本信息。
//
// 各变量通过 `-ldflags "-X ..."` 注入（见 Makefile）；默认值用于本地开发。
package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	// Version 是语义化版本或 git describe 结果，默认 "dev"。
	Version = "dev"
	// Commit 是构建对应的 git 短提交号，默认 "none"。
	Commit = "none"
	// Date 是构建时间（UTC, RFC3339），默认 "unknown"。
	Date = "unknown"
)

// Info 汇总一次构建的版本信息。
type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

// Get 返回当前构建信息。
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String 返回单行版本描述。
func (i Info) String() string {
	return fmt.Sprintf("autopcr %s (commit %s, built %s, %s, %s)",
		i.Version, i.Commit, i.Date, i.GoVersion, i.Platform)
}
