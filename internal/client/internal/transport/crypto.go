package transport

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// 固定 CBC 初始向量，复刻原 apiclient.py。
var cbcIV = []byte("7Fk9Lm3Np8Qr4Sv2")

const keyLen = 32 // AES-256；密钥为 32 个 ASCII 十六进制字符

// genKey 生成 32 字节密钥：每字节为随机十六进制字符的 ASCII 码
// （复刻 apiclient._createkey）。
func genKey() ([]byte, error) {
	const hexDigits = "0123456789abcdef"
	raw := make([]byte, keyLen)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	for i := range raw {
		raw[i] = hexDigits[int(raw[i])%len(hexDigits)]
	}
	return raw, nil
}

// pkcs7Pad 按 16 字节块做 PKCS#7 填充（复刻 apiclient._add_to_16）。
func pkcs7Pad(b []byte) []byte {
	n := aes.BlockSize - len(b)%aes.BlockSize
	return append(b, bytes.Repeat([]byte{byte(n)}, n)...)
}

// pkcs7Unpad 去除 PKCS#7 填充。异常输入按原样返回，避免 panic。
func pkcs7Unpad(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	n := int(b[len(b)-1])
	if n == 0 || n > len(b) {
		return b
	}
	return b[:len(b)-n]
}

func aesCBCEncrypt(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plain)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, cbcIV).CryptBlocks(out, padded)
	return out, nil
}

func aesCBCDecrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度 %d 非 %d 的整数倍", len(ciphertext), aes.BlockSize)
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, cbcIV).CryptBlocks(out, ciphertext)
	return out, nil
}

// encrypt 返回 aesCBC(pad(data)) || key（复刻 apiclient._encrypt）。
func encrypt(data, key []byte) ([]byte, error) {
	ct, err := aesCBCEncrypt(data, key)
	if err != nil {
		return nil, err
	}
	return append(ct, key...), nil
}

// decrypt 从 密文||key 中拆出密钥并解密，返回仍含 PKCS#7 填充的明文
// （复刻 apiclient._decrypt）。
func decrypt(data []byte) (plain, key []byte, err error) {
	if len(data) < keyLen {
		return nil, nil, fmt.Errorf("密文过短：%d < %d", len(data), keyLen)
	}
	key = data[len(data)-keyLen:]
	ciphertext := data[:len(data)-keyLen]
	plain, err = aesCBCDecrypt(ciphertext, key)
	return plain, key, err
}

// encryptViewerID 返回 base64(encrypt(str(viewerID), key))，用于加密请求的 viewer_id 字段。
func encryptViewerID(viewerID string, key []byte) (string, error) {
	enc, err := encrypt([]byte(viewerID), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// packCrypted 对请求体做 兼容 msgpack 编码后加密（复刻 apiclient._pack）。
func packCrypted(v any, key []byte) ([]byte, error) {
	body, err := marshalMsgpack(v)
	if err != nil {
		return nil, err
	}
	return encrypt(body, key)
}

// unpackCrypted 将响应体（base64 文本）解码并解密，返回去填充后的 msgpack 字节
// （复刻 apiclient._unpack）。
func unpackCrypted(b64 []byte) ([]byte, error) {
	// 直接解到自备缓冲，省掉 DecodeString 为 []byte→string 转换而做的整份拷贝（响应体可达数百 KB）。
	raw := make([]byte, base64.StdEncoding.DecodedLen(len(b64)))
	n, err := base64.StdEncoding.Decode(raw, b64)
	if err != nil {
		return nil, fmt.Errorf("响应 base64 解码失败: %w", err)
	}
	raw = raw[:n]
	plain, _, err := decrypt(raw)
	if err != nil {
		return nil, err
	}
	return pkcs7Unpad(plain), nil
}

// sidHash 计算 SID 头：md5(sid + "c!SID!n")（复刻 apiclient._request_internal）。
func sidHash(sid string) string {
	sum := md5.Sum([]byte(sid + "c!SID!n"))
	return hex.EncodeToString(sum[:])
}
