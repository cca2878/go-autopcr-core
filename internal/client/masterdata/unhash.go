package masterdata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// UnhashResult 概括一次反混淆的战果，供调用方判断这张 rainbow 还配不配得上这份母数据。
//
// 只统计到【表】这一级。列级的失配不在这里管：真实母数据里常年有几百个列没被 rainbow 覆盖
// （实测 202607 的库：10155 列中 353 个仍是哈希名，散在 58 张表里），拿它当信号只会天天误报；
// 而查到那些列的模块自己会报 no such column，那才是说得清是谁、缺什么的地方。
type UnhashResult struct {
	// Renamed 是还原成功的表数。
	Renamed int
	// Stale 是仍保留原名的表数（已排除 sqlite_ 内部表）。正常也不为零——rainbow 对新加的
	// 表总是慢一拍（实测同期基线为 3），故它是个「看趋势」的量，不是「非零即错」的开关。
	Stale int
	// StaleSample 是 Stale 里的头几个表名，供日志举例——全列出来会刷屏（失配时可达数百）。
	StaleSample []string
}

// staleSampleMax 是日志里最多举几个未还原表名。
const staleSampleMax = 3

// Unhash 依据 rainbow 把库中被哈希混淆的表名/列名还原为真实名，返回战果概览。
//
// 做法：把 rainbow 拍扁成一个全局 strings.Replacer，读一遍 sqlite_master，对每行的
// name/tbl_name/sql 文本一次性替换哈希名→真实名，再经 writable_schema 直接写回。
// 只遍历 schema（无数据拷贝、无逐列 ALTER 的整表 reparse），耗时亚秒级。
//
// 收尾时 bump schema_version 并 writable_schema=RESET：前者改动库头 cookie，让其它/
// 后续连接下次准备语句时重载 schema；后者让当前连接立即重载，从而同一 *sql.DB 句柄
// 在 Unhash 返回后即可用真实名查询。
//
// rainbow 未覆盖的列自动保留原名（不在替换表中即不动）。该操作是破坏性的：直接改写
// 传入的 db。对「落盘的干净库」执行一次即可。
func Unhash(db *sql.DB, rainbow Rainbow) (UnhashResult, error) {
	var res UnhashResult
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return res, err
	}
	defer func() { _ = conn.Close() }()

	// 一次性离线处理，无需崩溃安全：关闭 fsync 与回滚日志以提速。
	for _, p := range []string{"PRAGMA synchronous=OFF", "PRAGMA journal_mode=MEMORY"} {
		if _, err := conn.ExecContext(ctx, p); err != nil {
			return res, err
		}
	}

	var schemaVer int
	if err := conn.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&schemaVer); err != nil {
		return res, err
	}

	// 快照全部 schema 行（表/索引/触发器/视图）。
	type schemaRow struct {
		rowid          int64
		typ, name, tbl string
		sqlText        sql.NullString
	}
	var rows []schemaRow
	rs, err := conn.QueryContext(ctx, "SELECT rowid, type, name, tbl_name, sql FROM sqlite_master")
	if err != nil {
		return res, err
	}
	for rs.Next() {
		var r schemaRow
		if err := rs.Scan(&r.rowid, &r.typ, &r.name, &r.tbl, &r.sqlText); err != nil {
			_ = rs.Close()
			return res, err
		}
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		_ = rs.Close()
		return res, err
	}
	_ = rs.Close()

	repl := rainbow.replacer()

	if _, err := conn.ExecContext(ctx, "PRAGMA writable_schema=ON"); err != nil {
		return res, err
	}

	for _, r := range rows {
		newName := repl.Replace(r.name)
		newTbl := repl.Replace(r.tbl)
		newSQL := r.sqlText
		if r.sqlText.Valid {
			newSQL.String = repl.Replace(r.sqlText.String)
		}
		// 「反混淆前后各存一份表名再比对」不需要存两份：替换结果当场就能和原名比，这一步
		// 本来就在做（用来跳过无需改写的行），顺手把没动过的表记下来即零成本。
		if newName == r.name && newTbl == r.tbl && newSQL.String == r.sqlText.String {
			// sqlite_stat1/stat4 等内部表由 SQLite 自己维护，永远不带哈希名，不算失配。
			if r.typ == "table" && !strings.HasPrefix(r.name, "sqlite_") {
				res.Stale++
				if len(res.StaleSample) < staleSampleMax {
					res.StaleSample = append(res.StaleSample, r.name)
				}
			}
			continue
		}
		if _, err := conn.ExecContext(ctx,
			"UPDATE sqlite_master SET name=?, tbl_name=?, sql=? WHERE rowid=?",
			newName, newTbl, newSQL, r.rowid); err != nil {
			return res, fmt.Errorf("改写 schema 行 %q: %w", r.name, err)
		}
		if r.typ == "table" && newName != r.name {
			res.Renamed++
		}
	}

	// 改动 cookie 使 schema 变更被后续连接感知；RESET 让当前连接立即重载并关闭 writable_schema。
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA schema_version=%d", schemaVer+1)); err != nil {
		return res, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA writable_schema=RESET"); err != nil {
		if _, offErr := conn.ExecContext(ctx, "PRAGMA writable_schema=OFF"); offErr != nil {
			return res, err
		}
	}

	// 重命名只改了 sqlite_master 的 schema 文本，sqlite_stat1 的数据行里 tbl/idx 仍是旧
	// 哈希名，查询规划器无法匹配到真实表→统计失效。ANALYZE 清空重建为真实名统计。
	if _, err := conn.ExecContext(ctx, "ANALYZE"); err != nil {
		return res, fmt.Errorf("ANALYZE: %w", err)
	}
	return res, nil
}
