package masterdata

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
}

// NewManager 构造 Manager。cacheDir 下按 db/{ver}.db 缓存干净库。
func NewManager(cacheDir string, rainbow Rainbow, fetcher Fetcher) *Manager {
	return &Manager{cacheDir: cacheDir, rainbow: rainbow, fetcher: fetcher}
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
		return dbPath, nil
	}

	raw, err := m.fetcher.FetchMasterdata(ctx, ver)
	if err != nil {
		return "", fmt.Errorf("下载 masterdata(v%d): %w", ver, err)
	}
	sqliteBytes, err := unityfs.ExtractSQLite(raw)
	if err != nil {
		return "", fmt.Errorf("提取 SQLite: %w", err)
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// 临时文件名须【每次唯一】：多账号外壳共用一个 cacheDir 时，两次 EnsureDB 会同时构建同一
	// 版本；共用固定的 "<ver>.db.tmp" 会让二者互相截断——最坏情况是把尚未反混淆的库 rename
	// 成最终缓存，此后每次启动都命中这份坏缓存（查询全部报 no such table）。
	f, err := os.CreateTemp(dir, fmt.Sprintf("%d.db.*.tmp", ver))
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_, werr := f.Write(sqliteBytes)
	cerr := f.Close()
	if err := cmp.Or(werr, cerr); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := m.unhashFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("反混淆: %w", err)
	}
	// rename 是原子的：并发的两方各自把自己那份【已反混淆】的库落到同一目标，谁后到谁生效，
	// 两种结果都是完整可用的库。
	if err := os.Rename(tmp, dbPath); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dbPath, nil
}

func (m *Manager) unhashFile(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	_, err = Unhash(db, m.rainbow)
	return err
}
