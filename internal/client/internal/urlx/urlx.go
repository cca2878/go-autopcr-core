// Package urlx 提供 URL 拼接的统一约定：base URL 一律在解析阶段规整为「目录」形式
// （路径以 "/" 结尾），从而全项目可一致地用 (*url.URL).ResolveReference 拼接相对引用。
//
// 背景：ResolveReference 遵循 RFC 3986 相对解析——若 base 路径不以 "/" 结尾，解析相对
// 引用时会**丢弃 base 路径的最后一段**（如 base "/client_ob_771" + "Manifest/x" →
// "/Manifest/x"）。把规整放在解析阶段（ParseBase）即可根除该坑，调用点无需再关心。
//
// 约定：base 用 ParseBase 解析（保证尾斜杠）；相对引用（端点路径）不带前导 "/"。
package urlx

import (
	"net/url"
	"strings"
)

// ParseBase 解析一个 base URL，并确保其路径以 "/" 结尾（目录语义）。
func ParseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	ensureTrailingSlash(u)
	return u, nil
}

// MustParseBase 同 ParseBase，但解析失败即 panic；仅用于可信常量。
func MustParseBase(raw string) *url.URL {
	u, err := ParseBase(raw)
	if err != nil {
		panic("urlx: 非法 base URL " + raw + ": " + err.Error())
	}
	return u
}

func ensureTrailingSlash(u *url.URL) {
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		// 只有在 RawPath 本身不为空（即存在转义）时，才同步追加斜杠
		if u.RawPath != "" {
			u.RawPath += "/"
		}
	}
}
