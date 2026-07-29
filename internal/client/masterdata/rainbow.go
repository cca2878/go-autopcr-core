// Package masterdata 处理母数据 SQLite：反混淆（rainbow）与（后续）查询。
//
// CN 版 masterdata 的表名与列名被哈希混淆，data/rainbow.json 给出
// 哈希名→真实名 的映射。反混淆是一次性的：datagen 离线把干净库落盘，运行时
// 只读干净库、不再每次反混淆（见 Unhash）。
package masterdata

import (
	"encoding/json"
	"fmt"
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
