package masterdata

import (
	"context"
	"database/sql"
	"fmt"
)

// Unhash 依据 rainbow 把库中被哈希混淆的表名/列名还原为真实名，返回还原的表数量。
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
func Unhash(db *sql.DB, rainbow Rainbow) (int, error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	// 一次性离线处理，无需崩溃安全：关闭 fsync 与回滚日志以提速。
	for _, p := range []string{"PRAGMA synchronous=OFF", "PRAGMA journal_mode=MEMORY"} {
		if _, err := conn.ExecContext(ctx, p); err != nil {
			return 0, err
		}
	}

	var schemaVer int
	if err := conn.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&schemaVer); err != nil {
		return 0, err
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
		return 0, err
	}
	for rs.Next() {
		var r schemaRow
		if err := rs.Scan(&r.rowid, &r.typ, &r.name, &r.tbl, &r.sqlText); err != nil {
			_ = rs.Close()
			return 0, err
		}
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		_ = rs.Close()
		return 0, err
	}
	_ = rs.Close()

	repl := rainbow.replacer()

	if _, err := conn.ExecContext(ctx, "PRAGMA writable_schema=ON"); err != nil {
		return 0, err
	}

	count := 0
	for _, r := range rows {
		newName := repl.Replace(r.name)
		newTbl := repl.Replace(r.tbl)
		newSQL := r.sqlText
		if r.sqlText.Valid {
			newSQL.String = repl.Replace(r.sqlText.String)
		}
		if newName == r.name && newTbl == r.tbl && newSQL.String == r.sqlText.String {
			continue // 无哈希名，跳过
		}
		if _, err := conn.ExecContext(ctx,
			"UPDATE sqlite_master SET name=?, tbl_name=?, sql=? WHERE rowid=?",
			newName, newTbl, newSQL, r.rowid); err != nil {
			return count, fmt.Errorf("改写 schema 行 %q: %w", r.name, err)
		}
		if r.typ == "table" && newName != r.name {
			count++
		}
	}

	// 改动 cookie 使 schema 变更被后续连接感知；RESET 让当前连接立即重载并关闭 writable_schema。
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA schema_version=%d", schemaVer+1)); err != nil {
		return count, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA writable_schema=RESET"); err != nil {
		if _, offErr := conn.ExecContext(ctx, "PRAGMA writable_schema=OFF"); offErr != nil {
			return count, err
		}
	}

	// 重命名只改了 sqlite_master 的 schema 文本，sqlite_stat1 的数据行里 tbl/idx 仍是旧
	// 哈希名，查询规划器无法匹配到真实表→统计失效。ANALYZE 清空重建为真实名统计。
	if _, err := conn.ExecContext(ctx, "ANALYZE"); err != nil {
		return count, fmt.Errorf("ANALYZE: %w", err)
	}
	return count, nil
}
