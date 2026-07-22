// Package unityfs 从 UnityFS AssetBundle 中提取内嵌的 SQLite 母数据库，
// 不依赖 UnityPy，仅用纯 Go 的 LZ4（契合项目 CGO_ENABLED=0 硬约束）。
//
// 依据 ref/tools/unity3d_sqlite_extract.py（已对真实版本做字节级验证，输出与
// UnityPy 完全一致）。masterdata_master.unity3d 结构规整：UnityFS 容器全程用
// LZ4(block) 压缩，唯一节点是含单个 TextAsset 的 SerializedFile，其 m_Script
// 即原始 SQLite 文件字节。提取三步：
//
//  1. 解析 UnityFS 头 + blocksInfo（LZ4 解压得到块表）；
//  2. 逐块 LZ4 解压并拼接为 SerializedFile blob；
//  3. 在 blob 中按 SQLite 魔数 + 长度前缀取出 m_Script。
package unityfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// UnityFS flags（见 UnityPy BundleFile 定义）。
const (
	flagCompressionMask = 0x3f  // 低 6 位：压缩类型
	flagBlocksAtEnd     = 0x80  // blocksInfo 位于文件尾部
	flagPaddingAtStart  = 0x200 // 数据块起始需 16 字节对齐
)

// 压缩类型。
const (
	compNone  = 0
	compLZMA  = 1
	compLZ4   = 2
	compLZ4HC = 3
)

var sqliteMagic = []byte("SQLite format 3\x00")

type header struct {
	version              uint32
	compBlocksInfoSize   uint32
	uncompBlocksInfoSize uint32
	flags                uint32
	headerEnd            int // 头部（含 v>=7 的 16 字节对齐）结束偏移
}

type block struct {
	uncompressedSize uint32
	compressedSize   uint32
	flags            uint16
}

// blockEntrySize 是块表中每条记录的字节数（u32 + u32 + u16），用于校验不可信的 blockCount。
const blockEntrySize = 10

// maxLZ4Expansion 是 LZ4 block 格式的理论最大膨胀率（约 255:1）。解压前用它给不可信的
// uncompressedSize 设上限，避免坏文件声称的巨大长度直接变成一次巨额分配。
const maxLZ4Expansion = 255

// ExtractSQLite 从 UnityFS AssetBundle 字节中提取内嵌的 SQLite 数据库字节。
func ExtractSQLite(raw []byte) ([]byte, error) {
	h, err := parseHeader(raw)
	if err != nil {
		return nil, err
	}
	blocks, dataOff, err := parseBlocksInfo(raw, h)
	if err != nil {
		return nil, err
	}
	blob, err := decompressBlocks(raw, blocks, dataOff)
	if err != nil {
		return nil, err
	}
	return extractSQLiteFromBlob(blob)
}

func parseHeader(raw []byte) (*header, error) {
	r := newReader(raw)
	if sig := r.cstr(); sig != "UnityFS" {
		if r.err != nil {
			return nil, r.err
		}
		return nil, fmt.Errorf("不是 UnityFS 容器：signature=%q", sig)
	}
	h := &header{}
	h.version = r.u32()
	_ = r.cstr() // unity_version（未用）
	_ = r.cstr() // unity_revision（未用）
	_ = r.i64()  // size（未用）
	h.compBlocksInfoSize = r.u32()
	h.uncompBlocksInfoSize = r.u32()
	h.flags = r.u32()
	if h.version >= 7 {
		r.align16()
	}
	h.headerEnd = r.pos()
	if r.err != nil {
		return nil, r.err
	}
	return h, nil
}

func parseBlocksInfo(raw []byte, h *header) (blocks []block, dataOff int, err error) {
	// blocksInfo 位置
	var biOff int
	if h.flags&flagBlocksAtEnd != 0 {
		biOff = len(raw) - int(h.compBlocksInfoSize)
	} else {
		biOff = h.headerEnd
	}
	if biOff < 0 || biOff+int(h.compBlocksInfoSize) > len(raw) {
		return nil, 0, errors.New("blocksInfo 偏移越界")
	}

	bi, err := decompress(raw[biOff:biOff+int(h.compBlocksInfoSize)], int(h.uncompBlocksInfoSize), h.flags)
	if err != nil {
		return nil, 0, fmt.Errorf("解压 blocksInfo: %w", err)
	}
	if len(bi) != int(h.uncompBlocksInfoSize) {
		return nil, 0, fmt.Errorf("blocksInfo 解压后大小 %d != %d", len(bi), h.uncompBlocksInfoSize)
	}

	br := newReader(bi)
	br.take(16) // uncompressedDataHash（未用）
	blockCount := br.u32()
	// blockCount 来自文件内容，不可信：每条块表项 10 字节，超出剩余长度即为损坏数据。先校验再
	// 预分配，否则一个几十字节的坏文件就能让我们申请几十 GB（移动端直接被 OOM 杀掉）。
	if int64(blockCount)*blockEntrySize > int64(len(bi)-br.pos()) {
		return nil, 0, fmt.Errorf("块表项数 %d 超出 blocksInfo 剩余长度", blockCount)
	}
	blocks = make([]block, 0, blockCount)
	for range blockCount {
		blocks = append(blocks, block{br.u32(), br.u32(), br.u16()})
	}
	// node 表随后，提取时无需用到，跳过解析（仅校验读取未越界）。
	if br.err != nil {
		return nil, 0, fmt.Errorf("解析块表: %w", br.err)
	}

	// 数据块起始
	if h.flags&flagBlocksAtEnd != 0 {
		dataOff = h.headerEnd // blocksInfo 在尾部时，数据块紧接头部
	} else {
		dataOff = biOff + int(h.compBlocksInfoSize)
	}
	if h.flags&flagPaddingAtStart != 0 {
		dataOff = (dataOff + 15) &^ 15
	}
	return blocks, dataOff, nil
}

func decompressBlocks(raw []byte, blocks []block, dataOff int) ([]byte, error) {
	// 预分配用的总长同样来自不可信的块表：逐块核对「解压后大小 vs 压缩后大小」，并用 int64
	// 累加后再收口成 int，避免 32 位平台上溢出成负数（makeslice 会直接 panic）。
	var total int64
	for i, b := range blocks {
		if int64(b.uncompressedSize) > int64(b.compressedSize)*maxLZ4Expansion+16 {
			return nil, fmt.Errorf("数据块 %d 声称的解压后大小 %d 与压缩后大小 %d 不相称",
				i, b.uncompressedSize, b.compressedSize)
		}
		total += int64(b.uncompressedSize)
	}
	if total > int64(len(raw))*maxLZ4Expansion+16 {
		return nil, fmt.Errorf("块表声称的解压总长 %d 与文件长度 %d 不相称", total, len(raw))
	}
	blob := make([]byte, 0, total)
	cur := dataOff
	for i, blk := range blocks {
		end := cur + int(blk.compressedSize)
		if cur < 0 || end > len(raw) {
			return nil, fmt.Errorf("数据块 %d 越界（[%d:%d] / %d）", i, cur, end, len(raw))
		}
		part, err := decompress(raw[cur:end], int(blk.uncompressedSize), uint32(blk.flags))
		if err != nil {
			return nil, fmt.Errorf("解压数据块 %d: %w", i, err)
		}
		blob = append(blob, part...)
		cur = end
	}
	return blob, nil
}

// decompress 按压缩类型解压一个块 / blocksInfo。
func decompress(chunk []byte, uncompressedSize int, compFlag uint32) ([]byte, error) {
	switch compFlag & flagCompressionMask {
	case compNone:
		return chunk, nil
	case compLZ4, compLZ4HC:
		// Unity 用 LZ4 block 格式（非 frame），需显式给出解压后大小。该大小取自文件内容、
		// 不可信：先按 LZ4 的理论最大膨胀率核一遍，坏数据才不会变成一次巨额分配。
		if uncompressedSize < 0 || uncompressedSize > len(chunk)*maxLZ4Expansion+16 {
			return nil, fmt.Errorf("声称的解压后大小 %d 与压缩块长度 %d 不相称", uncompressedSize, len(chunk))
		}
		dst := make([]byte, uncompressedSize)
		n, err := lz4.UncompressBlock(chunk, dst)
		if err != nil {
			return nil, fmt.Errorf("lz4 解压: %w", err)
		}
		if n != uncompressedSize {
			return nil, fmt.Errorf("lz4 解压后大小 %d != %d", n, uncompressedSize)
		}
		return dst, nil
	case compLZMA:
		return nil, errors.New("该资源使用 LZMA 压缩块，本实现暂不支持（masterdata 未使用）")
	default:
		return nil, fmt.Errorf("未知压缩类型 %d", compFlag&flagCompressionMask)
	}
}

// extractSQLiteFromBlob 从 SerializedFile blob 中取出 TextAsset.m_Script（= SQLite 库）。
//
// m_Script 是 byte 数组，序列化为 [int32 长度(小端)] + [原始字节]；masterdata 的
// m_Script 首 16 字节必为 "SQLite format 3\0"。故定位魔数后，其前 4 字节即长度，
// 用于校验并切片。对本用例稳健且实现成本最低（无需完整解析 SerializedFile 类型树）。
func extractSQLiteFromBlob(blob []byte) ([]byte, error) {
	idx := bytes.Index(blob, sqliteMagic)
	if idx < 4 {
		return nil, errors.New("未在 blob 中找到 SQLite 魔数")
	}
	length := int(binary.LittleEndian.Uint32(blob[idx-4:]))
	end := idx + length
	if length < len(sqliteMagic) || end > len(blob) {
		return nil, fmt.Errorf("长度前缀 %d 超出 blob 范围（%d），疑似帧格式不符", length, len(blob))
	}
	// 拷贝出来，避免返回值继续引用整个大 blob。
	out := make([]byte, length)
	copy(out, blob[idx:end])
	return out, nil
}
