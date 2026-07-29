package unityfs

import (
	"encoding/binary"
	"fmt"
)

// reader 是带边界检查的大端二进制读取器（UnityFS 头/表均为大端）。
//
// 越界或格式错误时记录首个 err 并返回零值，调用方读取完毕后统一检查 err，
// 避免逐次判断也避免越界 panic。
type reader struct {
	b   []byte
	o   int
	err error
}

func newReader(b []byte) *reader { return &reader{b: b} }

func (r *reader) fail(format string, a ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w：%s", ErrMalformed, fmt.Sprintf(format, a...))
	}
}

// need 报告是否还能读取 n 字节。
func (r *reader) need(n int) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || r.o+n > len(r.b) {
		r.fail("读取越界：需要 %d 字节，偏移 %d，总长 %d", n, r.o, len(r.b))
		return false
	}
	return true
}

func (r *reader) u16() uint16 {
	if !r.need(2) {
		return 0
	}
	v := binary.BigEndian.Uint16(r.b[r.o:])
	r.o += 2
	return v
}

func (r *reader) u32() uint32 {
	if !r.need(4) {
		return 0
	}
	v := binary.BigEndian.Uint32(r.b[r.o:])
	r.o += 4
	return v
}

func (r *reader) i64() int64 {
	if !r.need(8) {
		return 0
	}
	v := int64(binary.BigEndian.Uint64(r.b[r.o:]))
	r.o += 8
	return v
}

// take 返回接下来 n 字节（引用底层切片，不拷贝）。
func (r *reader) take(n int) []byte {
	if !r.need(n) {
		return nil
	}
	v := r.b[r.o : r.o+n]
	r.o += n
	return v
}

// cstr 读取一个以 \0 结尾的字符串（不含 \0）。
func (r *reader) cstr() string {
	if r.err != nil {
		return ""
	}
	start := r.o
	for r.o < len(r.b) && r.b[r.o] != 0 {
		r.o++
	}
	if r.o >= len(r.b) {
		r.fail("cstr 未找到终止符 \\0（偏移 %d）", start)
		return ""
	}
	v := string(r.b[start:r.o])
	r.o++ // 跳过 \0
	return v
}

// align16 将偏移向上对齐到 16 字节。
func (r *reader) align16() { r.o = (r.o + 15) &^ 15 }

func (r *reader) pos() int { return r.o }
