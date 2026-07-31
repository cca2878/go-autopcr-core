//go:build ignore

// Command rainbow_gen 把 rainbow 源 JSON minify + gzip，写到内嵌用的目标路径。
// 由 Makefile 的 rainbow target 调用：go run rainbow_gen.go <源文件> <目标文件>。
//
// 源文件本身不进本仓库：更新频率低（游戏大版本才需要，约半年一次），Go 又没有编译期钩子能
// 校验「源文件」与「内嵌产物」是否同步，放一份进来除了膨胀仓库体积不解决任何问题——可读
// 版本与人工复核记录留在 ref/masterdata（工作区内的兄弟目录，不属于本仓）。
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法: go run rainbow_gen.go <源 rainbow.json> <目标 .json.gz>")
		os.Exit(2)
	}
	src, dst := os.Args[1], os.Args[2]

	raw, err := os.ReadFile(src)
	if err != nil {
		fatal(err)
	}

	var minified bytes.Buffer
	if err := json.Compact(&minified, raw); err != nil {
		fatal(fmt.Errorf("源文件不是合法 JSON: %w", err))
	}

	f, err := os.Create(dst)
	if err != nil {
		fatal(err)
	}
	defer f.Close()

	w, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		fatal(err)
	}
	if _, err := w.Write(minified.Bytes()); err != nil {
		fatal(err)
	}
	if err := w.Close(); err != nil {
		fatal(err)
	}
	fmt.Printf("%s (%d bytes) -> %s (minify+gzip, %d bytes)\n", src, len(raw), dst, minified.Len())
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}
