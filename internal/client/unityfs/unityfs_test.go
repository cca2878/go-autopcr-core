package unityfs

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/pierrec/lz4/v4"
)

// makeBlob 造一个含「长度前缀 + SQLite 内容」的 SerializedFile blob，返回 blob 与期望的 db。
func makeBlob() (blob, wantDB []byte) {
	content := append([]byte("SQLite format 3\x00"), bytes.Repeat([]byte("A"), 200)...)
	var buf bytes.Buffer
	buf.WriteString("PREFIX!!") // 8 字节，使魔数不在起始
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(content)))
	buf.Write(content)
	return buf.Bytes(), content
}

// buildBundle 造一个最小合法的 UnityFS(v7) 包，内含单块单节点，块数据 = blob。
func buildBundle(t *testing.T, blob []byte, compress bool) []byte {
	t.Helper()

	blockData := blob
	var blockFlags uint16 = compNone
	if compress {
		comp := make([]byte, lz4.CompressBlockBound(len(blob)))
		n, err := lz4.CompressBlock(blob, comp, nil)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Fatal("测试数据不可压缩，请换更可压缩的内容")
		}
		blockData = comp[:n]
		blockFlags = compLZ4
	}

	var bi bytes.Buffer
	bi.Write(make([]byte, 16)) // hash
	_ = binary.Write(&bi, binary.BigEndian, uint32(1))
	_ = binary.Write(&bi, binary.BigEndian, uint32(len(blob)))      // uncompressed_size
	_ = binary.Write(&bi, binary.BigEndian, uint32(len(blockData))) // compressed_size
	_ = binary.Write(&bi, binary.BigEndian, blockFlags)             // flags
	_ = binary.Write(&bi, binary.BigEndian, uint32(1))              // node_count
	_ = binary.Write(&bi, binary.BigEndian, int64(0))               // node offset
	_ = binary.Write(&bi, binary.BigEndian, int64(len(blob)))       // node size
	_ = binary.Write(&bi, binary.BigEndian, uint32(0))              // node flags
	bi.WriteString("CAB-test")
	bi.WriteByte(0)

	var hd bytes.Buffer
	hd.WriteString("UnityFS")
	hd.WriteByte(0)
	_ = binary.Write(&hd, binary.BigEndian, uint32(7)) // version
	hd.WriteString("5.x.x")
	hd.WriteByte(0)
	hd.WriteString("2021.3.20f1")
	hd.WriteByte(0)
	_ = binary.Write(&hd, binary.BigEndian, int64(0))         // size
	_ = binary.Write(&hd, binary.BigEndian, uint32(bi.Len())) // comp bi
	_ = binary.Write(&hd, binary.BigEndian, uint32(bi.Len())) // uncomp bi
	_ = binary.Write(&hd, binary.BigEndian, uint32(0))        // flags
	for hd.Len()%16 != 0 {                                    // v>=7 对齐
		hd.WriteByte(0)
	}

	var out bytes.Buffer
	out.Write(hd.Bytes())
	out.Write(bi.Bytes())
	out.Write(blockData)
	return out.Bytes()
}

func TestExtractSQLiteUncompressed(t *testing.T) {
	blob, want := makeBlob()
	got, err := ExtractSQLite(buildBundle(t, blob, false))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("提取结果不符：got %d 字节, want %d 字节", len(got), len(want))
	}
}

func TestExtractSQLiteLZ4(t *testing.T) {
	blob, want := makeBlob()
	got, err := ExtractSQLite(buildBundle(t, blob, true))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("LZ4 提取结果不符：got %d, want %d", len(got), len(want))
	}
}

func TestExtractSQLiteNotUnityFS(t *testing.T) {
	if _, err := ExtractSQLite([]byte("this is not a unityfs bundle at all")); err == nil {
		t.Fatal("非 UnityFS 输入应报错")
	}
}

func TestDecompressLZ4RoundTrip(t *testing.T) {
	orig := bytes.Repeat([]byte("hello masterdata "), 100)
	comp := make([]byte, lz4.CompressBlockBound(len(orig)))
	n, err := lz4.CompressBlock(orig, comp, nil)
	if err != nil || n == 0 {
		t.Fatalf("压缩失败 n=%d err=%v", n, err)
	}
	got, err := decompress(comp[:n], len(orig), compLZ4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatal("LZ4 往返不一致")
	}
}

func TestExtractFromBlobBadLength(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(1<<30)) // 超大长度前缀
	buf.Write([]byte("SQLite format 3\x00"))
	if _, err := extractSQLiteFromBlob(buf.Bytes()); err == nil {
		t.Fatal("越界长度前缀应报错")
	}
}
