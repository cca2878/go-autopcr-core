// Package mddb 是母数据各域查询共享的【低层只读 DB 句柄】。
//
// 类比 gameapi 的 transport：各域查询子包（masterdata/mission、masterdata/unit…）都只持有
// *mddb.DB 并在其上写自己的 SQL，从而顶层 masterdata 能以访问器聚合各域而不产生 import 环。
package mddb

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

// timeFormats 复刻 ref db.parse_time 支持的时间字符串格式（Go 参考布局，非零填充亦可解析）。
var timeFormats = []string{
	"2006/1/2 15:4:5",
	"2006/1/2 15:4",
	"2006/1/2",
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
	"20060102150405",
}

// ParseTime 解析母数据里的时间：先按 Unix 秒（纯数字），否则按已知格式（本地时区，与 ref 一致）。
// 各域共享此实现（时间格式在母数据里跨表一致），避免散落多份解析。
func ParseTime(s string) (time.Time, error) {
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	for _, f := range timeFormats {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &time.ParseError{Value: s, Message: ": 无法解析母数据时间"}
}

// DB 是干净母数据库的只读连接（以 mode=ro 打开，天然只读）。
type DB struct {
	sql *sql.DB
}

// Open 以只读模式打开 path 处的干净母数据库。
func Open(path string) (*DB, error) {
	sdb, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	if err := sdb.Ping(); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("打开母数据库 %s: %w", path, err)
	}
	return &DB{sql: sdb}, nil
}

// Close 释放底层连接。
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// QueryContext 执行只读多行查询（调用方负责 Close 返回的 *sql.Rows）。
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.sql.QueryContext(ctx, query, args...)
}

// QueryRowContext 执行只读单行查询。
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.sql.QueryRowContext(ctx, query, args...)
}

// LoadIntSet 执行返回单列整型 id 的查询，把结果并入 set（各域装配 id 集合的常用助手）。
func (d *DB) LoadIntSet(ctx context.Context, set map[int]struct{}, query string, args ...any) error {
	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		set[id] = struct{}{}
	}
	return rows.Err()
}
