// Package appversion 把【上一次探测到的真实 APP-VER】落盘缓存，避免每个新进程都要靠一次被
// 拒绝的请求才能自愈（见 transport.Client 的 store_url 自愈 + WithOnVersionUpdate）。
//
// cacheDir 为空即整包空操作：CLI 一类每次都是全新进程的外壳，接上它就不必每次都吃一次多余
// 往返；没有缓存目录的调用方仍靠 transport.Client 的单次自愈兜底，只是每次都要重新触发一次。
//
// 有意做成一个独立的小文件而非某种通用缓存抽象：现在只有这一个使用者，母数据缓存
// （masterdata.Manager）的目录结构/淘汰策略与这里的诉求也不同，猜测将来的形状不如等真的出现
// 第二个需求时再抽象。
package appversion

import (
	"os"
	"path/filepath"
	"strings"
)

const filename = "version.txt"

// Read 读出缓存的版本号。cacheDir 为空、文件不存在或内容为空都返回 ok=false——调用方据此
// 判断「有没有可用的缓存值」，不必关心具体原因。
func Read(cacheDir string) (ver string, ok bool) {
	if cacheDir == "" {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(cacheDir, filename))
	if err != nil {
		return "", false
	}
	v := strings.TrimSpace(string(b))
	return v, v != ""
}

// Write 把版本号原子落盘：先写临时文件再 rename，避免另一进程并发读到写了一半的内容
// （同一 cacheDir 下的母数据库缓存曾因非原子写入吃过亏，见 masterdata.Manager.EnsureDB）。
// 失败静默吞掉——这只是个旁路优化，不该让它反过来影响调用方那次本已成功的请求。
func Write(cacheDir, ver string) {
	if cacheDir == "" {
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	tmp := filepath.Join(cacheDir, filename+".tmp")
	if err := os.WriteFile(tmp, []byte(ver), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(cacheDir, filename))
}
