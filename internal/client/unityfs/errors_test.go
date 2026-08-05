package unityfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 提取失败必须落到 ErrMalformed 上——母数据构建链据此判断"这份下载坏了，重来一次有戏"。
// 只断言 err != nil 是不够的：那样把哨兵摘掉也照样绿。
func TestMalformedInputsAreClassifiedCorrupt(t *testing.T) {
	badLength := func() []byte {
		var buf bytes.Buffer
		_ = binary.Write(&buf, binary.LittleEndian, uint32(1<<30))
		buf.Write([]byte("SQLite format 3\x00"))
		return buf.Bytes()
	}()

	cases := []struct {
		name string
		run  func() error
	}{
		{"非 UnityFS 容器", func() error {
			_, err := ExtractSQLite([]byte("this is not a unityfs bundle at all"))
			return err
		}},
		{"头部被截断", func() error {
			_, err := ExtractSQLite([]byte("UnityFS\x00\x00\x00"))
			return err
		}},
		{"长度前缀越界", func() error {
			_, err := extractSQLiteFromBlob(badLength)
			return err
		}},
		{"blob 里没有 SQLite 魔数", func() error {
			_, err := extractSQLiteFromBlob(bytes.Repeat([]byte("x"), 64))
			return err
		}},
		{"未压缩块长度对不上", func() error {
			return decompressInto(make([]byte, 8), []byte("too short"), compNone)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.run()
			if err == nil {
				t.Fatal("应报错")
			}
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("应可 errors.Is 命中 ErrMalformed，得到 %v", err)
			}
			if got := errs.Classify(err).Kind; got != errs.KindCorrupt {
				t.Errorf("Classify = %v, want KindCorrupt", got)
			}
		})
	}
}

// "我们没实现这个特性"与"数据坏了"是两码事：前者重下多少次都一样，故必须分得开。
func TestUnsupportedCompressionIsNotMalformed(t *testing.T) {
	cases := map[string]uint32{
		"LZMA":   compLZMA,
		"未知压缩类型": 0x3f,
	}
	for name, flag := range cases {
		t.Run(name, func(t *testing.T) {
			err := decompressInto(make([]byte, 4), []byte("data"), flag)
			if err == nil {
				t.Fatal("应报错")
			}
			if !errors.Is(err, ErrUnsupported) {
				t.Errorf("应命中 ErrUnsupported，得到 %v", err)
			}
			if errors.Is(err, ErrMalformed) {
				t.Error("不支持的特性不应被当成数据损坏——那会让上层白白重下一遍")
			}
			if got := errs.Classify(err).Kind; got != errs.KindUnsupported {
				t.Errorf("Classify = %v, want KindUnsupported", got)
			}
		})
	}
}
