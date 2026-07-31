package appversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if _, ok := Read(dir, "APP-VER"); ok {
		t.Fatal("空目录不应有缓存值")
	}

	Write(dir, "APP-VER", "11.7.2")
	got, ok := Read(dir, "APP-VER")
	if !ok || got != "11.7.2" {
		t.Fatalf("Read = (%q, %v), want (\"11.7.2\", true)", got, ok)
	}

	// tmp 文件必须被 rename 掉，不留痕迹。
	if _, err := os.Stat(filepath.Join(dir, filename+".tmp")); !os.IsNotExist(err) {
		t.Errorf("临时文件应已被 rename 清除，err=%v", err)
	}

	Write(dir, "APP-VER", "11.7.3")
	if got, _ := Read(dir, "APP-VER"); got != "11.7.3" {
		t.Errorf("二次 Write 后 Read = %q, want \"11.7.3\"（应覆盖）", got)
	}
}

// 两个键暂时存进同一个文件：写一个键不该抹掉另一个键已有的值。
func TestWritePreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()

	Write(dir, "APP-VER", "11.7.2")
	Write(dir, "RES-VER", "10002200")

	if got, ok := Read(dir, "APP-VER"); !ok || got != "11.7.2" {
		t.Errorf("APP-VER 被后续写入 RES-VER 影响，got=(%q,%v)", got, ok)
	}
	if got, ok := Read(dir, "RES-VER"); !ok || got != "10002200" {
		t.Errorf("RES-VER = (%q,%v), want (\"10002200\", true)", got, ok)
	}

	Write(dir, "APP-VER", "11.7.3")
	if got, ok := Read(dir, "RES-VER"); !ok || got != "10002200" {
		t.Errorf("覆盖 APP-VER 后 RES-VER 应保持不变，got=(%q,%v)", got, ok)
	}
}

func TestEmptyCacheDirIsNoop(t *testing.T) {
	Write("", "APP-VER", "11.7.2") // 不应 panic 或报错
	if _, ok := Read("", "APP-VER"); ok {
		t.Error("空 cacheDir 应始终返回 ok=false")
	}
}

func TestReadIgnoresWhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(dir, "APP-VER"); ok {
		t.Error("空白内容应视为无缓存值")
	}
}

func TestReadUnknownKey(t *testing.T) {
	dir := t.TempDir()
	Write(dir, "APP-VER", "11.7.2")
	if _, ok := Read(dir, "RES-VER"); ok {
		t.Error("未写过的键应返回 ok=false")
	}
}
