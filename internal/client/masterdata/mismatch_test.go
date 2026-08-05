package masterdata

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/errs"
	_ "modernc.org/sqlite"
)

// hashedPackage 造一个"表名列名都是哈希"的母数据包，表名取自 tables。
func hashedPackage(t *testing.T, tables ...string) []byte {
	t.Helper()
	srcPath := filepath.Join(t.TempDir(), "src.db")
	db, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	for _, name := range tables {
		mustExec(t, db, "CREATE TABLE "+name+" (c_"+name+" INTEGER)")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	return wrapUnityFS(raw)
}

// logAt 造一个按指定级别收集输出的 logger。
func logAt(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})), &buf
}

// 一张表都没还原＝这份 rainbow 配不上这个版本的母数据。过去这里静默通过：库照常落盘、日志
// 打"构建完成"、登录成功，直到跑模块才逐个炸出 no such table，根因早已无从追溯。
func TestTotalMismatchFailsAtBuildTime(t *testing.T) {
	logger, buf := logAt(slog.LevelDebug)
	cacheDir := t.TempDir()
	mgr := NewManager(cacheDir,
		Rainbow{"v1_oldhash": {tableNameKey: "unit_data", "c_v1_oldhash": "unit_id"}},
		&fakeFetcher{data: hashedPackage(t, "v1_newhash")},
		WithManagerLogger(logger))

	_, err := mgr.EnsureDB(context.Background(), 42)
	if err == nil {
		t.Fatal("rainbow 全不匹配时必须报错，不能静默产出一个查不动的库")
	}
	if !errors.Is(err, ErrRainbowMismatch) {
		t.Errorf("应可 errors.Is 命中 ErrRainbowMismatch，得到 %v", err)
	}

	// 归类必须说清"是哪一部分、该怎么办"：母数据域 + 本版认不出（升级客户端），而不是
	// 数据损坏（那会引导用户徒劳地清缓存重下）。
	if got, want := errs.Classify(err), errs.DomainMasterdata.With(errs.KindUnsupported); got != want {
		t.Errorf("分类 = %v，want %v", got, want)
	}

	// 坏库绝不能落盘：留下它，后续每次启动都会命中这份查不动的缓存。
	if _, serr := os.Stat(mgr.DBPath(42)); serr == nil {
		t.Error("失配时不该留下缓存库")
	}
	if strings.Contains(buf.String(), "构建完成") {
		t.Errorf("失败的构建不该报告「构建完成」，日志：\n%s", buf.String())
	}
}

// 部分表没还原不阻断——缺的表未必有模块要查。但要 Warn 说出来，并带上数字：stale=3 是常态
// 基线，stale=800 一眼就知道是换包了。
func TestPartialMismatchWarnsButProceeds(t *testing.T) {
	logger, buf := logAt(slog.LevelDebug)
	mgr := NewManager(t.TempDir(),
		Rainbow{"v1_known": {tableNameKey: "unit_data"}},
		&fakeFetcher{data: hashedPackage(t, "v1_known", "v1_unknown_a", "v1_unknown_b")},
		WithManagerLogger(logger))

	if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
		t.Fatalf("部分失配不该阻断构建：%v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "level=WARN") {
		t.Errorf("未还原的表应记 Warn，日志：\n%s", got)
	}
	for _, want := range []string{"stale=2", "renamed=1"} {
		if !strings.Contains(got, want) {
			t.Errorf("Warn 应带数字 %q，日志：\n%s", want, got)
		}
	}
}

// 全部还原时不该有 Warn——没有瑕疵就没什么可提醒的。
func TestCleanUnhashDoesNotWarn(t *testing.T) {
	logger, buf := logAt(slog.LevelDebug)
	mgr := NewManager(t.TempDir(),
		Rainbow{"v1_known": {tableNameKey: "unit_data"}},
		&fakeFetcher{data: hashedPackage(t, "v1_known")},
		WithManagerLogger(logger))

	if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("全部还原时不该 Warn，日志：\n%s", buf.String())
	}
}

// sqlite_stat1/stat4 由 SQLite 自己维护、永远不带哈希名，把它们算进 stale 会平白多出两条
// 恒定的告警。ANALYZE 就在 Unhash 收尾处，故这两张表必然存在。
func TestInternalTablesAreNotCountedStale(t *testing.T) {
	db := openTempDB(t)
	mustExec(t, db, "CREATE TABLE v1_known (a INTEGER)")
	mustExec(t, db, "INSERT INTO v1_known VALUES (1)")
	mustExec(t, db, "ANALYZE") // 先造出 sqlite_stat1

	res, err := Unhash(db, Rainbow{"v1_known": {tableNameKey: "unit_data"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stale != 0 {
		t.Errorf("SQLite 内部表不该算作未还原，Stale=%d sample=%v", res.Stale, res.StaleSample)
	}
}

// 指纹必须只由内容决定。Go 的 map 迭代顺序是随机的，忘了排序就会每次算出不同的值——缓存
// 于是永远判为不匹配，用户每次登录重下几十 MB，而且这种 bug 在单次运行里看不出来。
func TestFingerprintIsStableAcrossIterations(t *testing.T) {
	rb := Rainbow{}
	for _, tbl := range []string{"v1_a", "v1_b", "v1_c", "v1_d", "v1_e"} {
		rb[tbl] = map[string]string{tableNameKey: tbl + "_real", "c1": "x", "c2": "y", "c3": "z"}
	}
	want := rb.Fingerprint()
	for i := range 100 {
		if got := rb.Fingerprint(); got != want {
			t.Fatalf("第 %d 次指纹漂移：%d != %d（map 迭代顺序没被排序抹平）", i, got, want)
		}
	}
}

func TestFingerprintDiffersOnContentChange(t *testing.T) {
	a := Rainbow{"v1_a": {tableNameKey: "unit_data", "c1": "unit_id"}}
	b := Rainbow{"v1_a": {tableNameKey: "unit_data", "c1": "unit_name"}}
	if a.Fingerprint() == b.Fingerprint() {
		t.Error("内容不同的 rainbow 不该有相同指纹")
	}
}

// 换了 rainbow 而 manifest_ver 没动时，缓存必须重建。这正是"换包后我们发新版修好 rainbow"
// 的场景：只看文件在不在的话，用户会一直吃那份用旧表建出来的库，发多少版都救不回来。
func TestCacheRebuiltWhenRainbowChanges(t *testing.T) {
	cacheDir := t.TempDir()
	pkg := hashedPackage(t, "v1_tab")
	const ver = 42

	ff1 := &fakeFetcher{data: pkg}
	old := NewManager(cacheDir, Rainbow{"v1_tab": {tableNameKey: "old_name"}}, ff1)
	if _, err := old.EnsureDB(context.Background(), ver); err != nil {
		t.Fatal(err)
	}

	ff2 := &fakeFetcher{data: pkg}
	fixed := NewManager(cacheDir, Rainbow{"v1_tab": {tableNameKey: "unit_data"}}, ff2)
	path, err := fixed.EnsureDB(context.Background(), ver)
	if err != nil {
		t.Fatal(err)
	}
	if ff2.calls != 1 {
		t.Errorf("换了 rainbow 应重建（fetch 1 次），实际 fetch %d 次", ff2.calls)
	}

	// 重建后查得到新 rainbow 的表名，说明拿到的确实是新库而非旧缓存。
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM unit_data").Scan(&n); err != nil {
		t.Errorf("重建后应能按新 rainbow 的表名查询：%v", err)
	}
}

// 反过来同样要钉住：rainbow 没变就不许重建。指纹一旦不稳定，这里会红——那意味着每次登录
// 都白下一遍几十 MB。
func TestCacheKeptWhenRainbowUnchanged(t *testing.T) {
	cacheDir := t.TempDir()
	pkg := hashedPackage(t, "v1_tab")
	rb := Rainbow{"v1_tab": {tableNameKey: "unit_data"}}
	ff := &fakeFetcher{data: pkg}

	for i := range 3 {
		mgr := NewManager(cacheDir, rb, ff)
		if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
			t.Fatalf("第 %d 次 EnsureDB: %v", i, err)
		}
	}
	if ff.calls != 1 {
		t.Errorf("rainbow 未变时只该构建一次，实际 fetch %d 次", ff.calls)
	}
}

// 缓存文件损坏（不是个 SQLite 库）时读不出指纹，应当重建而不是把坏文件一路带到 Open 才炸。
func TestCorruptCacheIsRebuilt(t *testing.T) {
	cacheDir := t.TempDir()
	mgr := NewManager(cacheDir, Rainbow{"v1_tab": {tableNameKey: "unit_data"}},
		&fakeFetcher{data: hashedPackage(t, "v1_tab")})

	dbPath := mgr.DBPath(42)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := mgr.EnsureDB(context.Background(), 42)
	if err != nil {
		t.Fatalf("损坏的缓存应被重建而非报错：%v", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM unit_data").Scan(&n); err != nil {
		t.Errorf("重建后的库应可用：%v", err)
	}
}
