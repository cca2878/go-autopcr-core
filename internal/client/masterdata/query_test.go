package masterdata

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQuery(t *testing.T) {
	// 造一个含 unit_data 的干净库。
	path := filepath.Join(t.TempDir(), "clean.db")
	sdb, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	sdb.SetMaxOpenConns(1)
	mustExec(t, sdb, "CREATE TABLE unit_data (unit_id INTEGER PRIMARY KEY, unit_name TEXT NOT NULL)")
	mustExec(t, sdb, "INSERT INTO unit_data VALUES (100101, '日和莉'), (100201, '优衣')")
	_ = sdb.Close()

	q, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = q.Close() }()

	ctx := context.Background()
	name, err := q.Unit().Name(ctx, 100101)
	if err != nil {
		t.Fatalf("Unit().Name: %v", err)
	}
	if name != "日和莉" {
		t.Fatalf("Unit().Name=%q want 日和莉", name)
	}

	n, err := q.Unit().Count(ctx)
	if err != nil {
		t.Fatalf("Unit().Count: %v", err)
	}
	if n != 2 {
		t.Fatalf("Unit().Count=%d want 2", n)
	}

	// 不存在的 unit_id 返回 ErrNoRows。
	if _, err := q.Unit().Name(ctx, 999999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("缺失 unit_id 应返回 ErrNoRows，得 %v", err)
	}
}
