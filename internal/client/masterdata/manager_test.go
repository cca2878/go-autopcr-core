package masterdata

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"log/slog"
	_ "modernc.org/sqlite"
	"strings"
)

type fakeFetcher struct {
	data  []byte
	calls int
}

func (f *fakeFetcher) FetchMasterdata(ctx context.Context, ver int) ([]byte, error) {
	f.calls++
	return f.data, nil
}

// wrapUnityFS 把 sqlite 字节包成一个最小的 UnityFS(v7, compNone) 包。
func wrapUnityFS(sqliteBytes []byte) []byte {
	var blob bytes.Buffer
	blob.WriteString("PREFIX!!")
	_ = binary.Write(&blob, binary.LittleEndian, uint32(len(sqliteBytes)))
	blob.Write(sqliteBytes)
	blobBytes := blob.Bytes()

	var bi bytes.Buffer
	bi.Write(make([]byte, 16))
	_ = binary.Write(&bi, binary.BigEndian, uint32(1))
	_ = binary.Write(&bi, binary.BigEndian, uint32(len(blobBytes)))
	_ = binary.Write(&bi, binary.BigEndian, uint32(len(blobBytes)))
	_ = binary.Write(&bi, binary.BigEndian, uint16(0))
	_ = binary.Write(&bi, binary.BigEndian, uint32(1))
	_ = binary.Write(&bi, binary.BigEndian, int64(0))
	_ = binary.Write(&bi, binary.BigEndian, int64(len(blobBytes)))
	_ = binary.Write(&bi, binary.BigEndian, uint32(0))
	bi.WriteString("CAB-test")
	bi.WriteByte(0)

	var hd bytes.Buffer
	hd.WriteString("UnityFS")
	hd.WriteByte(0)
	_ = binary.Write(&hd, binary.BigEndian, uint32(7))
	hd.WriteString("5.x")
	hd.WriteByte(0)
	hd.WriteString("2021.3.20f1")
	hd.WriteByte(0)
	_ = binary.Write(&hd, binary.BigEndian, int64(0))
	_ = binary.Write(&hd, binary.BigEndian, uint32(bi.Len()))
	_ = binary.Write(&hd, binary.BigEndian, uint32(bi.Len()))
	_ = binary.Write(&hd, binary.BigEndian, uint32(0))
	for hd.Len()%16 != 0 {
		hd.WriteByte(0)
	}

	var out bytes.Buffer
	out.Write(hd.Bytes())
	out.Write(bi.Bytes())
	out.Write(blobBytes)
	return out.Bytes()
}

func TestManagerEnsureDB(t *testing.T) {
	// 造含哈希表的真实 sqlite，取其字节。
	srcPath := filepath.Join(t.TempDir(), "src.db")
	sdb, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatal(err)
	}
	sdb.SetMaxOpenConns(1)
	mustExec(t, sdb, "CREATE TABLE hashtab (hcol1 INTEGER, plaincol TEXT)")
	mustExec(t, sdb, "INSERT INTO hashtab VALUES (7, 'z')")
	_ = sdb.Close()
	sqliteBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	ff := &fakeFetcher{data: wrapUnityFS(sqliteBytes)}
	rainbow := Rainbow{"hashtab": {tableNameKey: "realtab", "hcol1": "id"}}
	mgr := NewManager(t.TempDir(), rainbow, ff)

	path, err := mgr.EnsureDB(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if path != mgr.DBPath(42) {
		t.Fatalf("path=%q want %q", path, mgr.DBPath(42))
	}

	// 打开干净库，校验反混淆后的表/列/数据。
	cdb, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cdb.Close() }()
	var id int
	var plain string
	if err := cdb.QueryRow("SELECT id, plaincol FROM realtab").Scan(&id, &plain); err != nil {
		t.Fatalf("查询干净库: %v", err)
	}
	if id != 7 || plain != "z" {
		t.Fatalf("数据错误: id=%d plain=%q", id, plain)
	}

	// 再次 EnsureDB：命中缓存，不应重复下载。
	if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if ff.calls != 1 {
		t.Fatalf("缓存未命中，fetcher 调用 %d 次", ff.calls)
	}
}

// 构建母数据库要下载几十 MB、解包、再改写整个库的 schema，是首次登录里最久的一段。
// 全程静默的话，用户看到的就是长时间无响应——故这段路径必须在'默认级别'说话。
func TestEnsureDBReportsProgress(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.db")
	sdb, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatal(err)
	}
	sdb.SetMaxOpenConns(1)
	mustExec(t, sdb, "CREATE TABLE hashtab (hcol1 INTEGER)")
	_ = sdb.Close()
	sqliteBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	mgr := NewManager(t.TempDir(), Rainbow{"hashtab": {tableNameKey: "realtab"}},
		&fakeFetcher{data: wrapUnityFS(sqliteBytes)}, WithManagerLogger(logger))

	if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
		t.Fatal(err)
	}

	// Info 级别下要能看出"开始了"和"完成了"，否则等待期间无从判断是否卡死。
	got := buf.String()
	for _, want := range []string{"开始构建", "构建完成", "ver=42"} {
		if !strings.Contains(got, want) {
			t.Errorf("Info 级日志应包含 %q，实际输出：\n%s", want, got)
		}
	}

	// 第二次是缓存命中，这条路径不该再刷 Info——那会让每次登录都多两行噪音。
	buf.Reset()
	if _, err := mgr.EnsureDB(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("缓存命中不该产生 Info 级日志，实际输出：\n%s", buf.String())
	}
}
