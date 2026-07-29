package masterdata

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// touch 在缓存的 db 目录里造一个文件，并可指定修改时间。
func touch(t *testing.T, dir, name string, age time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		mt := time.Now().Add(-age)
		if err := os.Chtimes(path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func newPruneFixture(t *testing.T) (*Manager, string) {
	t.Helper()
	cacheDir := t.TempDir()
	m := NewManager(cacheDir, Rainbow{}, nil)
	dbDir := filepath.Dir(m.DBPath(0))
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return m, dbDir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// 旧版本库不会再被打开（本库任何时候只用最新 manifest_ver），每个 42MB 上下，不清就是
// 无上限累积。
func TestPruneRemovesOlderVersions(t *testing.T) {
	m, dbDir := newPruneFixture(t)
	old1 := touch(t, dbDir, "202606011200.db", 0)
	old2 := touch(t, dbDir, "202607011238.db", 0)
	cur := touch(t, dbDir, "202607290900.db", 0)

	m.pruneCache(202607290900)

	if exists(old1) || exists(old2) {
		t.Error("比当前版本旧的库应被清掉")
	}
	if !exists(cur) {
		t.Fatal("当前版本的库绝不能被清掉")
	}
}

// 服务端回滚时当前版本会小于目录里已有的。那些更新的库可能正被另一个进程持有、或马上就要
// 再用，不该由我们代为判废。
func TestPruneKeepsNewerVersionsOnRollback(t *testing.T) {
	m, dbDir := newPruneFixture(t)
	older := touch(t, dbDir, "202606011200.db", 0)
	newer := touch(t, dbDir, "202607290900.db", 0)

	m.pruneCache(202607011238) // 回滚到中间某版

	if exists(older) {
		t.Error("更旧的仍应清掉")
	}
	if !exists(newer) {
		t.Error("比 keep 新的版本不该被清掉")
	}
}

// 孤儿临时文件按【年龄】判废：另一个进程可能正在同一目录里构建，我们看不见它进行到哪一步，
// 年龄是唯一不需要跨进程协调的判据。
func TestPruneOnlyRemovesStaleTempFiles(t *testing.T) {
	m, dbDir := newPruneFixture(t)
	fresh := touch(t, dbDir, "202607290900.db.123.tmp", time.Minute)
	stale := touch(t, dbDir, "202607290900.db.456.tmp", staleTempAge+time.Hour)

	m.pruneCache(202607290900)

	if !exists(fresh) {
		t.Error("刚创建的临时文件可能是别人正在构建的，不得删除")
	}
	if exists(stale) {
		t.Error("超龄的孤儿临时文件应被清掉")
	}
}

// 目录里混着的其它东西（子目录、无关文件、版本号解析不出来的名字）一律不碰。
func TestPruneIgnoresUnrelatedEntries(t *testing.T) {
	m, dbDir := newPruneFixture(t)
	notes := touch(t, dbDir, "readme.txt", 0)
	weird := touch(t, dbDir, "backup.db", 0) // 名字不是纯数字，解析不出版本
	sub := filepath.Join(dbDir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	m.pruneCache(202607290900)

	for _, p := range []string{notes, weird, sub} {
		if !exists(p) {
			t.Errorf("不该动 %s", filepath.Base(p))
		}
	}
}

// 目录不存在时（首次运行、还没建过任何库）必须安然返回——清理绝不能影响 EnsureDB 的成败。
func TestPruneOnMissingDirIsNoop(t *testing.T) {
	m := NewManager(t.TempDir(), Rainbow{}, nil)
	m.pruneCache(1) // 不 panic 即可
}

// EnsureDB 命中缓存时也要清理：用户长期停在同一版本时，旧库同样该被清掉。
func TestEnsureDBPrunesOnCacheHit(t *testing.T) {
	m, dbDir := newPruneFixture(t)
	const cur = 202607290900
	touch(t, dbDir, strconv.Itoa(cur)+".db", 0)
	old := touch(t, dbDir, "202606011200.db", 0)

	path, err := m.EnsureDB(t.Context(), cur)
	if err != nil {
		t.Fatalf("命中缓存不应报错：%v", err)
	}
	if path != m.DBPath(cur) {
		t.Errorf("path = %q", path)
	}
	if exists(old) {
		t.Error("命中缓存的路径上也应清理旧版本")
	}
}
