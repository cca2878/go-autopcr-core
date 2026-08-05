package unityfs

import "github.com/cca2878/go-autopcr-core/internal/errs"

// 本包的两类失败。分开它们是为了回答调用方唯一关心的那个问题：'重下一份还有没有救'。
//
// 母数据构建链上，提取失败的最常见成因是下载残缺或缓存损坏（ErrMalformed，重来一次通常
// 就好），而非我们碰上了没实现的容器特性（ErrUnsupported，重来多少次都一样）。二者混在
// 一个泛型错误里，上层就只能一律放弃或一律重试。
// 归 DomainMasterdata 而非自立一域：本包是母数据链上的一环（资源包解包），它的错误只会经
// masterdata.BuildError 冒到调用方那里，而调用方对它的处置——清掉这份下载重来——和母数据链
// 上其它环节没有区别。Domain 按处置动作划分，不按包划分。
var (
	// ErrMalformed 表示输入字节不是合法的 UnityFS 内容：签名不符、头/块表越界、长度前缀
	// 对不上、解压结果与声明的大小不一致等。
	ErrMalformed = errs.DomainMasterdata.New(errs.KindCorrupt, "UnityFS 数据损坏")

	// ErrUnsupported 表示内容本身可能合法，但用到了本实现没有覆盖的特性（如 LZMA 压缩块）。
	ErrUnsupported = errs.DomainMasterdata.New(errs.KindUnsupported, "UnityFS 特性不支持")
)
