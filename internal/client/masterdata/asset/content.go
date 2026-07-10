// Package asset 负责从 B 服 patch CDN 解析资源清单并下载资源包。
//
// 对应原 Python 项目 db/assetmgr.py。清单（manifest）是层级 CSV：顶层清单列出
// 若干子清单与资源条目，子清单再列出更多条目；每个资源条目携带 md5，实际文件位于
// pool/{category}/{md5[:2]}/{md5}。
package asset

import "strings"

// Content 是清单中的一个条目。
type Content struct {
	URL      string
	MD5      string
	Type     string
	Category string
}

// isManifest 报告该条目是否为子清单（url 以 "manifest/" 开头）。
func (c *Content) isManifest() bool { return strings.HasPrefix(c.URL, "manifest/") }

// parseLine 解析一行清单 CSV：url,md5,type,size（部分行有额外偏移列）。
// 复刻原 content.from_line。无法解析时返回 ok=false。
func parseLine(line, category string) (*Content, bool) {
	line = strings.TrimRight(line, "\r")
	splits := strings.Split(line, ",")
	// 至少需要 url,md5,type,size 四列。
	if len(splits) < 4 {
		return nil, false
	}
	offset := 0
	if len(splits) > 5 {
		offset = 1
	}
	if 2+offset >= len(splits) {
		return nil, false
	}
	return &Content{
		URL:      splits[0],
		MD5:      splits[1],
		Type:     splits[2+offset],
		Category: category,
	}, true
}
