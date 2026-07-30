package masterdata

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestUnhash(t *testing.T) {
	db := openTempDB(t)
	// 混淆表：hcol1/hcol2 需还原，plaincol 未在 rainbow 中（保留原名）。
	mustExec(t, db, "CREATE TABLE hashtab (hcol1 INTEGER, hcol2 TEXT, plaincol TEXT)")
	mustExec(t, db, "INSERT INTO hashtab VALUES (1, 'alice', 'x')")
	mustExec(t, db, "INSERT INTO hashtab VALUES (2, 'bob', 'y')")

	rainbow := Rainbow{
		"hashtab": {
			tableNameKey:  "realtab",
			"hcol1":       "id",
			"hcol2":       "name",
			"hcol_absent": "ghost", // 版本漂移：该列不存在，应被忽略且不影响其余
		},
	}

	res, err := Unhash(db, rainbow)
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 1 {
		t.Fatalf("还原表数=%d want 1", res.Renamed)
	}

	// 真实表存在且列名/数据正确（含保留原名的 plaincol）。
	var id int
	var name, plain string
	if err := db.QueryRow("SELECT id, name, plaincol FROM realtab WHERE id=2").Scan(&id, &name, &plain); err != nil {
		t.Fatalf("查询 realtab: %v", err)
	}
	if id != 2 || name != "bob" || plain != "y" {
		t.Fatalf("数据错误: id=%d name=%q plain=%q", id, name, plain)
	}

	// 原哈希表名已不存在（被重命名走）。
	if tableExists(t, db, "hashtab") {
		t.Fatal("哈希表名 hashtab 不应仍存在")
	}
}

func TestUnhashSkipsAbsentTable(t *testing.T) {
	db := openTempDB(t)
	rainbow := Rainbow{
		"not_in_db": {tableNameKey: "whatever", "a": "b"},
	}
	res, err := Unhash(db, rainbow)
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 0 {
		t.Fatalf("不存在的表不应被还原，Renamed=%d", res.Renamed)
	}
}
