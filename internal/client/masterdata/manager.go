package masterdata

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cca2878/go-autopcr-core/internal/client/unityfs"
)

// Fetcher 下载指定版本的 masterdata_master.unity3d 原始字节（由 asset.Source 实现）。
type Fetcher interface {
	FetchMasterdata(ctx context.Context, ver int) ([]byte, error)
}

// Manager 按版本管理干净母数据库：检测→下载→提取→反混淆→落盘缓存。
//
// 运行时从登录得到的 manifest_ver 调用 EnsureDB(ver)：已缓存则秒回；版本变动时
// 自动构建新版本的干净库。反混淆一次性完成并落盘，之后只读、不再反混淆。
type Manager struct {
	cacheDir string
	rainbow  Rainbow
	fetcher  Fetcher
	logger   *slog.Logger
}

// ManagerOption 定制 Manager。
type ManagerOption func(*Manager)

// WithManagerLogger 设置日志器（默认 slog.Default()）。
func WithManagerLogger(l *slog.Logger) ManagerOption {
	return func(m *Manager) {
		if l != nil {
			m.logger = l
		}
	}
}

// NewManager 构造 Manager。cacheDir 下按 db/{ver}.db 缓存干净库。
func NewManager(cacheDir string, rainbow Rainbow, fetcher Fetcher, opts ...ManagerOption) *Manager {
	m := &Manager{cacheDir: cacheDir, rainbow: rainbow, fetcher: fetcher, logger: slog.Default()}
	for _, o := range opts {
		o(m)
	}
	return m
}

// DBPath 返回版本 ver 干净库的缓存路径（不保证已存在）。
func (m *Manager) DBPath(ver int) string {
	return filepath.Join(m.cacheDir, "db", fmt.Sprintf("%d.db", ver))
}

// EnsureDB 确保版本 ver 的干净库已就绪并返回其路径。
//
// 已缓存则直接返回；否则：下载 unity3d → 提取 SQLite → 反混淆 → 原子落盘
// （先写 .tmp 再 rename，避免半成品被误用）。
func (m *Manager) EnsureDB(ctx context.Context, ver int) (string, error) {
	dbPath := m.DBPath(ver)
	if _, err := os.Stat(dbPath); err == nil {
		m.pruneCache(ver)
		return dbPath, nil
	}

	raw, err := m.fetcher.FetchMasterdata(ctx, ver)
	if err != nil {
		return "", buildErr(ver, StageDownload, err)
	}
	sqliteBytes, err := unityfs.ExtractSQLite(raw)
	if err != nil {
		return "", buildErr(ver, StageExtract, err)
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", buildErr(ver, StageStore, err)
	}
	// 临时文件名须【每次唯一】：多账号外壳共用一个 cacheDir 时，两次 EnsureDB 会同时构建同一
	// 版本；共用固定的 "<ver>.db.tmp" 会让二者互相截断——最坏情况是把尚未反混淆的库 rename
	// 成最终缓存，此后每次启动都命中这份坏缓存（查询全部报 no such table）。
	f, err := os.CreateTemp(dir, fmt.Sprintf("%d.db.*.tmp", ver))
	if err != nil {
		return "", buildErr(ver, StageStore, err)
	}
	tmp := f.Name()
	_, werr := f.Write(sqliteBytes)
	cerr := f.Close()
	if err := cmp.Or(werr, cerr); err != nil {
		_ = os.Remove(tmp)
		return "", buildErr(ver, StageStore, err)
	}
	if err := m.unhashFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", buildErr(ver, StageUnhash, err)
	}
	// rename 是原子的：并发的两方各自把自己那份【已反混淆】的库落到同一目标，谁后到谁生效，
	// 两种结果都是完整可用的库。
	if err := os.Rename(tmp, dbPath); err != nil {
		_ = os.Remove(tmp)
		return "", buildErr(ver, StageStore, err)
	}
	m.pruneCache(ver)
	return dbPath, nil
}

// staleTempAge 是孤儿临时文件的判废年龄。取值须【远大于】一次正常构建的耗时：构建中的
// 临时文件也在同一个目录里，按年龄区分是唯一不需要跨进程协调的判据（同一 cacheDir 可能
// 被另一个进程的外壳同时使用，我们看不见它的构建进行到哪一步）。
const staleTempAge = 24 * time.Hour

// pruneCache 清掉缓存目录里已无用的东西：比 keep 旧的版本库，以及久未改动的孤儿临时文件。
//
// 为什么可以删：本库任何时候都只用最新的 manifest_ver，旧版本库不会再被打开——每个 42MB
// 上下，不清理就是无上限累积（移动端尤其吃不消）。
//
// 为什么只删【更旧】的：版本号回退时（服务端回滚）当前版本会小于目录里已有的，这时那些
// 更新的库仍可能被另一个进程持有或马上再用，不该由我们代为判废。
//
// 为什么删失败可以不管：另一个进程正持有该文件时，Windows 会拒绝删除（POSIX 上删掉也不影响
// 它已打开的句柄）。这只意味着这次没清掉，下次 EnsureDB 会再试——清理是尽力而为，绝不能
// 让它影响 EnsureDB 的成败。
func (m *Manager) pruneCache(keep int) {
	dir := filepath.Dir(m.DBPath(keep))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var removed, freed int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		victim := false
		switch {
		case strings.HasSuffix(name, ".tmp"):
			// 孤儿临时文件：正常路径会自行删除，留下来的是上次进程崩在半途的残骸。
			info, ierr := e.Info()
			victim = ierr == nil && time.Since(info.ModTime()) > staleTempAge
		case strings.HasSuffix(name, ".db"):
			ver, cerr := strconv.Atoi(strings.TrimSuffix(name, ".db"))
			victim = cerr == nil && ver < keep
		}
		if !victim {
			continue
		}

		size := int64(0)
		if info, ierr := e.Info(); ierr == nil {
			size = info.Size()
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			continue
		}
		removed++
		freed += size
		// SQLite 的附属文件（只读模式下通常不存在，但库若为 WAL 模式则可能留下）。
		for _, suffix := range []string{"-wal", "-shm"} {
			_ = os.Remove(filepath.Join(dir, name+suffix))
		}
	}
	if removed > 0 {
		m.logger.Info("已清理旧的母数据缓存",
			"files", removed, "freed_mb", freed/(1<<20), "keep_ver", keep)
	}
}

// unhashFile 就地反混淆 path 处的库。Close 的错误必须上报而非吞掉：紧随其后的 rename 会把
// 这份文件变成永久缓存，若收尾时刷盘失败却当成功，坏库会一直被后续启动命中。
func (m *Manager) unhashFile(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	_, err = Unhash(db, m.rainbow)
	return cmp.Or(err, db.Close())
}
