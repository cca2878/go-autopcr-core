// Package masterdata 处理母数据 SQLite：反混淆（rainbow）与只读查询（见 Reader）。
//
// CN 版 masterdata 的表名与列名被哈希混淆，rainbow.json（随客户端 go:embed 内嵌，见
// internal/client/masterdata.go）给出哈希名→真实名的映射。反混淆按版本一次性完成并落盘：
// Manager.EnsureDB 检测到新版本时下载、提取、反混淆并写入干净库，之后同一版本只读该库、
// 不再重复反混淆（见 Unhash）。
package masterdata

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// tableNameKey 是 rainbow 表项中指向真实表名的特殊键。
const tableNameKey = "--table_name"

// Rainbow 是 rainbow.json 的结构：
//
//	哈希表名 -> { "--table_name": 真实表名, 哈希列名: 真实列名, ... }
type Rainbow map[string]map[string]string

// ParseRainbow 解析 rainbow.json。
func ParseRainbow(data []byte) (Rainbow, error) {
	var r Rainbow
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadRainbow, err)
	}
	return r, nil
}

// Fingerprint 是这张 rainbow 的内容指纹，用来标记"某个干净库是用哪张表反混淆出来的"。
//
// 为什么要它：干净库按 db/{ver}.db 缓存，而缓存键里没有 rainbow 的份。换了 rainbow 却撞上
// 同一个 ver 时，EnsureDB 会命中那份用旧表建出来的库并直接返回——修好 rainbow 发了新版也
// 救不回来，除非用户手动删缓存。指纹补上的正是这一维。
//
// 取 SHA256 前 4 字节而非全量，是为了塞进 SQLite 的 user_version（int32，见 Manager 的
// stampFingerprint）。这里防的是"版本对不上"而不是攻击，候选集只有寥寥几张历史 rainbow，
// 4 字节足够；即便真撞上，代价也不过是少重建一次。
//
// 遍历 map 前先排序：Go 的 map 迭代顺序是随机的，不排序则同一张表每次算出的指纹都不同，
// 缓存会永远判为不匹配、每次登录重下几十 MB。
func (r Rainbow) Fingerprint() int32 {
	h := sha256.New()
	tables := make([]string, 0, len(r))
	for t := range r {
		tables = append(tables, t)
	}
	slices.Sort(tables)
	for _, t := range tables {
		cols := make([]string, 0, len(r[t]))
		for c := range r[t] {
			cols = append(cols, c)
		}
		slices.Sort(cols)
		h.Write([]byte(t))
		for _, c := range cols {
			h.Write([]byte(c))
			h.Write([]byte(r[t][c]))
		}
	}
	return int32(binary.BigEndian.Uint32(h.Sum(nil)[:4])) //nolint:gosec // 有意截断取指纹
}

// replacer 把整张 rainbow 拍扁为一个哈希名→真实名 的全局替换器。
//
// 表名与列名合并进同一映射：哈希名唯一，故可安全地一次性替换 sqlite_master 里的
// name/tbl_name/sql 文本。同一哈希列名（如 "id" 的哈希）会在多张表里重复出现且
// 映射到相同真实名，用 map 去重避免 Replacer 内部 trie 膨胀。
func (r Rainbow) replacer() *strings.Replacer {
	m := make(map[string]string)
	for hashedTable, cols := range r {
		if intact := cols[tableNameKey]; intact != "" {
			m[hashedTable] = intact
		}
		for hashedCol, intactCol := range cols {
			if hashedCol == tableNameKey {
				continue
			}
			m[hashedCol] = intactCol
		}
	}
	pairs := make([]string, 0, len(m)*2)
	for hashed, real := range m {
		if hashed == real {
			continue
		}
		pairs = append(pairs, hashed, real)
	}
	return strings.NewReplacer(pairs...)
}
