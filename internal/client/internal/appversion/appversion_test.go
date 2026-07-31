package appversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if _, ok := Read(dir); ok {
		t.Fatal("空目录不应有缓存值")
	}

	Write(dir, "11.7.2")
	got, ok := Read(dir)
	if !ok || got != "11.7.2" {
		t.Fatalf("Read = (%q, %v), want (\"11.7.2\", true)", got, ok)
	}

	// tmp 文件必须被 rename 掉，不留痕迹。
	if _, err := os.Stat(filepath.Join(dir, filename+".tmp")); !os.IsNotExist(err) {
		t.Errorf("临时文件应已被 rename 清除，err=%v", err)
	}

	Write(dir, "11.7.3")
	if got, _ := Read(dir); got != "11.7.3" {
		t.Errorf("二次 Write 后 Read = %q, want \"11.7.3\"（应覆盖）", got)
	}
}

func TestEmptyCacheDirIsNoop(t *testing.T) {
	Write("", "11.7.2") // 不应 panic 或报错
	if _, ok := Read(""); ok {
		t.Error("空 cacheDir 应始终返回 ok=false")
	}
}

func TestReadIgnoresWhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(dir); ok {
		t.Error("空白内容应视为无缓存值")
	}
}
