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
		// 版本号相同不代表这份缓存还能用：它可能是'另一张 rainbow'建出来的。换包后我们发新版
		// 修好 rainbow，而 manifest_ver 未必跟着动，此时若只看文件在不在，用户会一直吃那份用旧
		// 表建出来的库——发多少版都救不回来，除非他自己去删缓存。
		want := m.rainbow.Fingerprint()
		switch got, ferr := cacheFingerprint(dbPath); {
		case ferr != nil:
			m.logger.Warn("缓存的母数据库读不出 rainbow 指纹，按需重建", "ver", ver, "err", ferr)
		case got == want:
			m.logger.Debug("母数据库已就绪", "ver", ver, "path", dbPath)
			m.pruneCache(ver)
			return dbPath, nil
		default:
			m.logger.Info("缓存的母数据库出自另一张 rainbow，重建",
				"ver", ver, "cached_fp", got, "want_fp", want)
		}
	}

	// 这条是 Info 而非 Debug：下面三步要下载几十 MB、解包、再改写整个库的 schema，首次登录
	// 时是全流程里最久的一段。不说一声，用户看到的就是长时间无响应。
	m.logger.Info("母数据库尚未缓存，开始构建", "ver", ver)
	started := time.Now()

	raw, err := m.fetcher.FetchMasterdata(ctx, ver)
	if err != nil {
		return "", buildErr(ver, StageDownload, err)
	}
	m.logger.Debug("母数据资源包下载完成", "ver", ver, "bytes", len(raw), "elapsed", time.Since(started))

	extractAt := time.Now()
	sqliteBytes, err := unityfs.ExtractSQLite(raw)
	if err != nil {
		return "", buildErr(ver, StageExtract, err)
	}
	m.logger.Debug("SQLite 提取完成", "ver", ver, "bytes", len(sqliteBytes), "elapsed", time.Since(extractAt))

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", buildErr(ver, StageStore, err)
	}
	// 临时文件名须'每次唯一'：多账号外壳共用一个 cacheDir 时，两次 EnsureDB 会同时构建同一
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
	unhashAt := time.Now()
	if err := m.unhashFile(tmp, ver); err != nil {
		_ = os.Remove(tmp)
		return "", buildErr(ver, StageUnhash, err)
	}
	m.logger.Debug("反混淆完成", "ver", ver, "elapsed", time.Since(unhashAt))
	// rename 是原子的：并发的两方各自把自己那份'已反混淆'的库落到同一目标，谁后到谁生效，
	// 两种结果都是完整可用的库。
	if err := os.Rename(tmp, dbPath); err != nil {
		_ = os.Remove(tmp)
		return "", buildErr(ver, StageStore, err)
	}
	m.logger.Info("母数据库构建完成", "ver", ver, "size_mb", len(sqliteBytes)/(1<<20),
		"elapsed", time.Since(started))
	m.pruneCache(ver)
	return dbPath, nil
}

// staleTempAge 是孤儿临时文件的判废年龄。取值须'远大于'一次正常构建的耗时：构建中的
// 临时文件也在同一个目录里，按年龄区分是唯一不需要跨进程协调的判据（同一 cacheDir 可能
// 被另一个进程的外壳同时使用，我们看不见它的构建进行到哪一步）。
const staleTempAge = 24 * time.Hour

// pruneCache 清掉缓存目录里已无用的东西：比 keep 旧的版本库，以及久未改动的孤儿临时文件。
//
// 为什么可以删：本库任何时候都只用最新的 manifest_ver，旧版本库不会再被打开——每个 42MB
// 上下，不清理就是无上限累积（移动端尤其吃不消）。
//
// 为什么只删'更旧'的：版本号回退时（服务端回滚）当前版本会小于目录里已有的，这时那些
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

// unhashFile 就地反混淆 path 处的库，校验战果，并盖上 rainbow 指纹。
//
// Close 的错误必须上报而非吞掉：紧随其后的 rename 会把这份文件变成永久缓存，若收尾时刷盘
// 失败却当成功，坏库会一直被后续启动命中。
func (m *Manager) unhashFile(path string, ver int) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	res, err := Unhash(db, m.rainbow)
	if err != nil {
		return cmp.Or(err, db.Close())
	}
	if err := m.checkUnhash(ver, res); err != nil {
		// 这条路径上 Close 的错误可以丢：调用方收到错误就会删掉这个临时文件，它不会变成缓存，
		// 上面那句"Close 错误必须上报"的理由在这里不成立。
		_ = db.Close()
		return err
	}
	return cmp.Or(m.stampFingerprint(db), db.Close())
}

// checkUnhash 判读反混淆战果。
//
// 一张都没还原 → 硬失败。过去这里是'静默通过'的最大的一个洞：还原表数被丢弃，没反混淆的库
// 照常落盘、日志还打一条"构建完成"，登录也成功，直到跑模块才逐个炸出 no such table——而那时
// 错误已经和根因隔了十万八千里，用户只看得到一句 SQL 报错。
//
// 还剩表没还原（但不是全部）→ Warn，照常继续。不设阈值、有几张报几张：这条只在'构建新版本
// 母数据'时才走到（命中缓存根本不到这里），一天顶多响几次，当得起每次都提醒一遍；而 rainbow
// 没盖全本就是实打实的瑕疵，值得看见。数字自己会说话——`stale=3` 是常态基线，`stale=800` 一眼
// 就知道换包了。
//
// 缺的表未必有模块要查，故不阻断；真查到了，模块自己会报 no such table，那才是说得清是谁、
// 缺什么的地方。
func (m *Manager) checkUnhash(ver int, res UnhashResult) error {
	if res.Renamed == 0 && len(m.rainbow) > 0 {
		return fmt.Errorf("%w：v%d 的 %d 张表无一还原（内嵌 rainbow 覆盖 %d 张表）",
			ErrRainbowMismatch, ver, res.Stale, len(m.rainbow))
	}
	if res.Stale > 0 {
		m.logger.Warn("母数据有表未能反混淆，rainbow 未覆盖到它们",
			"ver", ver, "renamed", res.Renamed, "stale", res.Stale, "sample", res.StaleSample)
		return nil
	}
	m.logger.Debug("母数据反混淆战果", "ver", ver, "renamed", res.Renamed)
	return nil
}

// stampFingerprint 把 rainbow 指纹写进库的 user_version（SQLite 文件头里的 32 位应用自定义
// 字段），让这份缓存自带"我是谁建的"。
//
// 为什么不写进文件名：DBPath 的 {ver}.db 是 pruneCache 唯一的判据——它靠 Atoi 解析文件名来
// 认出旧版本库，名字里多一段指纹会让解析失败、于是永远不删，几十 MB 一份地漏下去。写在库内部
// 则文件名不变，清理逻辑一个字都不用动。
//
// user_version 而非 application_id：后者的语义是"这是什么类型的文件"，该是个跨版本的常量；
// 前者本就是留给应用自己编版本号的。pragma 不支持参数绑定，故用 Sprintf（与 Unhash 里
// schema_version 的写法一致）。
func (m *Manager) stampFingerprint(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.rainbow.Fingerprint()))
	return err
}

// cacheFingerprint 读出 path 处缓存库上盖的 rainbow 指纹。只读打开，读的是文件头、不碰表。
//
// 本次改动之前建的缓存没盖过章，读出来是 SQLite 的默认值 0。那和"指纹不符"同样处置——重建。
// 来路不明的缓存就该重建，代价是升级到本版后各用户会多下一次母数据。
func cacheFingerprint(path string) (int32, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()
	var v int32
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
