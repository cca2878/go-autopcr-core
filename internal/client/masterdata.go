package client

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"

	"github.com/cca2878/go-autopcr-core/internal/client/masterdata"
)

// embeddedRainbow 是随客户端编译进二进制的默认反混淆表，gzip 压缩存放。
//
// rainbow 归无头客户端所有：游戏每次大更新（更换 APK 包体）时全量变化，首期以内嵌
// 静态资产承载，不做版本化/热更。将来热更落地时可演进为「内嵌默认 + 下载版本覆盖」。
//
// 实测（2026-07-31，909 张表的版本）：原始 JSON 1.0MB，gzip -9 压到 442KB（42.6%）；
// 项目里已用的 lz4（UnityFS 解包用它是因为解压快，见 internal/client/unityfs）压缩比反而
// 明显更差（Level9 也只到 60.9%）——这里的取舍和那里相反：rainbow 只在启动时解一次，压缩比
// 比解压速度重要得多，故选 gzip。
//
// 只有压缩后的 rainbow.json.gz 进本仓库，可读源文件不进：更新频率低（游戏大版本才需要，约
// 半年一次），Go 又没有编译期钩子能校验「源文件」与「内嵌产物」是否同步，放一份进来除了
// 膨胀仓库体积不解决任何问题。可读版本与人工复核记录留在 ref/masterdata（工作区内的兄弟
// 目录）。更新时用 `make rainbow SRC=/path/to/rainbow.json` 重新生成本文件。
//
//go:embed rainbow.json.gz
var embeddedRainbowGz []byte

// defaultRainbow 解压并解析内嵌的默认反混淆表。
func defaultRainbow() (masterdata.Rainbow, error) {
	r, err := gzip.NewReader(bytes.NewReader(embeddedRainbowGz))
	if err != nil {
		return nil, fmt.Errorf("解压内嵌 rainbow: %w", err)
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("解压内嵌 rainbow: %w", err)
	}
	return masterdata.ParseRainbow(data)
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
