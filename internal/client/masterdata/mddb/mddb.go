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

	"github.com/cca2878/go-autopcr-core/internal/errs"
	_ "modernc.org/sqlite"
)

// ErrOpenDB 表示已构建好的母数据库打不开——文件被删、权限不对、或落盘时就已损坏。
// 归 KindEnvironment：这台机器上的事，重新下载一次母数据通常能修好。
var ErrOpenDB = errs.DomainMasterdata.New(errs.KindEnvironment, "打开母数据库失败")

// timeFormats 复刻 ref db.parse_time 支持的时间字符串格式（Go 参考布局，非零填充亦可解析）。
var timeFormats = []string{
	"2006/1/2 15:4:5",
	"2006/1/2 15:4",
	"2006/1/2",
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
	"20060102150405",
}

// serverZone 是母数据时间字符串的时区：母数据记的是国服挂钟时间（UTC+8）。ref 靠部署环境把
// 本机时区钉成 Asia/Shanghai 才等价，而本仓是库、不能假设宿主时区，故显式固定，否则 UTC 主机上
// 所有活动窗口都会偏 8 小时，且同一账号在不同时区主机上的模块输出不一致（违反确定性）。
var serverZone = time.FixedZone("CST", 8*60*60)

// ParseTime 解析母数据里的时间：先按 Unix 秒（纯数字），否则按已知格式（国服时区 UTC+8）。
// 各域共享此实现（时间格式在母数据里跨表一致），避免散落多份解析。
func ParseTime(s string) (time.Time, error) {
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	for _, f := range timeFormats {
		if t, err := time.ParseInLocation(f, s, serverZone); err == nil {
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
		return nil, fmt.Errorf("%w %s: %w", ErrOpenDB, path, err)
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
