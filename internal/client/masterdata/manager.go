package masterdata

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cca2878/go-autopcr/internal/client/unityfs"
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

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return "", err
	}
	tmp := dbPath + ".tmp"
	if err := os.WriteFile(tmp, sqliteBytes, 0o644); err != nil {
		return "", err
	}
	if err := m.unhashFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("反混淆: %w", err)
	}
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
