package transport

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/ugorji/go/codec"
)

// 固定 32 字节（AES-256）密钥，仅用于测试。
var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestPKCS7RoundTrip(t *testing.T) {
	for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 33} {
		data := bytes.Repeat([]byte{0xAB}, n)
		padded := pkcs7Pad(data)
		if len(padded)%16 != 0 {
			t.Fatalf("n=%d: padded len %d not block-aligned", n, len(padded))
		}
		if got := pkcs7Unpad(padded); !bytes.Equal(got, data) {
			t.Fatalf("n=%d: unpad got %x want %x", n, got, data)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	data := []byte("hello 公主连结 payload with 一些中文")
	enc, err := encrypt(data, testKey)
	if err != nil {
		t.Fatal(err)
	}
	plain, key, err := decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, testKey) {
		t.Fatalf("extracted key mismatch")
	}
	if got := pkcs7Unpad(plain); !bytes.Equal(got, data) {
		t.Fatalf("decrypted %q want %q", got, data)
	}
}

func TestGenKeyIsHex(t *testing.T) {
	const hexDigits = "0123456789abcdef"
	k, err := genKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != keyLen {
		t.Fatalf("key len %d want %d", len(k), keyLen)
	}
	for _, b := range k {
		if !bytes.ContainsRune([]byte(hexDigits), rune(b)) {
			t.Fatalf("non-hex byte %q in key", b)
		}
	}
}

// TestPackUnpackCryptedRoundTrip 覆盖「请求打包 → 服务器 base64 回传 → 响应解包」。
func TestPackUnpackCryptedRoundTrip(t *testing.T) {
	type body struct {
		A string `msgpack:"a"`
		N int    `msgpack:"n"`
	}
	raw, err := packCrypted(body{A: "值", N: 7}, testKey)
	if err != nil {
		t.Fatal(err)
	}
	// 服务器侧：把加密结果 base64。
	b64 := base64.StdEncoding.EncodeToString(raw)

	mp, err := unpackCrypted([]byte(b64))
	if err != nil {
		t.Fatal(err)
	}
	var got body
	if err := codec.NewDecoderBytes(mp, responseHandle).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.A != "值" || got.N != 7 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSidHashDeterministic(t *testing.T) {
	a := sidHash("abc")
	b := sidHash("abc")
	if a != b {
		t.Fatal("sidHash 不确定")
	}
	if len(sidHash("abc")) != 32 {
		t.Fatalf("sidHash 长度 %d want 32", len(sidHash("abc")))
	}
}
