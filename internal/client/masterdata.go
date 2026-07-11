package client

import (
	_ "embed"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
)

// embeddedRainbow 是随客户端编译进二进制的默认反混淆表。
//
// rainbow 归无头客户端所有：游戏每次大更新（更换 APK 包体）时全量变化，首期以内嵌
// 静态资产承载，不做版本化/热更。将来热更落地时可演进为「内嵌默认 + 下载版本覆盖」。
//
//go:embed rainbow.json
var embeddedRainbow []byte

// defaultRainbow 解析内嵌的默认反混淆表。
func defaultRainbow() (masterdata.Rainbow, error) {
	return masterdata.ParseRainbow(embeddedRainbow)
}

// NewMasterdataRefresher 用内嵌 rainbow 装配一个母数据 Refresher。
//
// 供【无需登录/凭证】地确保或刷新母数据：调用其 Refresh(ctx) 即自行握手取最新版本并落库，
// 保证程序总能拿到并展示最新数据。rainbow 归客户端所有，故由客户端包注入、对上层保持封装。
func NewMasterdataRefresher(cacheDir string, opts ...masterdata.RefresherOption) (*masterdata.Refresher, error) {
	rainbow, err := defaultRainbow()
	if err != nil {
		return nil, err
	}
	return masterdata.NewRefresher(cacheDir, rainbow, opts...), nil
}
