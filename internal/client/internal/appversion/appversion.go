// Package appversion 把【服务端权威下发的、会随游戏更新变化的请求头值】（APP-VER、RES-VER）
// 落盘缓存，避免每个新进程都要从零重新探测。
//
// APP-VER 靠 transport.Client 的 store_url 自愈探测（见其 transport 方法 +
// WithOnVersionUpdate）；RES-VER 没有对应的拒绝信号，握手成功的响应本身就带着权威值
// （discovery.Result.ResVer），故直接在拿到后原样写入即可，不需要自愈重试那一套。
//
// cacheDir 为空即整包空操作：CLI 一类每次都是全新进程的外壳，接上它就不必每次都从头探测；
// 没有缓存目录的调用方仍能正常工作，只是每次都要重新走一遍探测/自愈。
//
// 两个键暂时存进同一个文件（key=value 逐行），没有做成通用缓存抽象：现在只有这两个使用者，
// 母数据缓存（masterdata.Manager）的目录结构/淘汰策略与这里的诉求也不同，猜测将来的形状
// 不如等真的需要时再抽象。
package appversion

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const filename = "version.txt"

// Read 读出 cacheDir 下缓存的 key 对应值。cacheDir 为空、文件不存在、key 不存在或值为空都
// 返回 ok=false——调用方据此判断「有没有可用的缓存值」，不必关心具体原因。
func Read(cacheDir, key string) (value string, ok bool) {
	if cacheDir == "" {
		return "", false
	}
	m := readAll(cacheDir)
	v, ok := m[key]
	return v, ok && v != ""
}

// Write 把 key=value 原子落盘，保留文件里其余已有的键（读-改-写）。先写临时文件再 rename，
// 避免另一进程并发读到写了一半的内容（同一 cacheDir 下的母数据库缓存曾因非原子写入吃过亏，
// 见 masterdata.Manager.EnsureDB）。失败静默吞掉——这只是个旁路优化，不该让它反过来影响
// 调用方那次本已成功的请求。
func Write(cacheDir, key, value string) {
	if cacheDir == "" {
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	m := readAll(cacheDir)
	if m == nil {
		m = make(map[string]string)
	}
	m[key] = value

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys) // 稳定输出，避免每次落盘顺序不同造成无意义的文件内容变化

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
		b.WriteByte('\n')
	}

	tmp := filepath.Join(cacheDir, filename+".tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(cacheDir, filename))
}

// readAll 解析 cacheDir 下的缓存文件；不存在或读不出就当作空（nil）。
func readAll(cacheDir string) map[string]string {
	raw, err := os.ReadFile(filepath.Join(cacheDir, filename))
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[k] = v
	}
	return m
}
