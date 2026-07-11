package transport

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/ugorji/go/codec"
)

// 事实（官方权威实现）：客户端请求仍用【旧】msgpack 规范，服务端响应已用【新】规范。
// 故编码与解码使用两个不同配置的 handle。

// requestHandle 用于编码请求体——旧规范（等价 Python packb(use_bin_type=False)）：
//   - WriteExt=false：字符串走老式 raw 家族（fixstr / raw16=0xda / raw32=0xdb），不用 str8/bin；
//   - TypeInfos=["msgpack"]：读取结构体 `msgpack` tag。
//
// 无 omitempty 的字段全部发送，nil 指针编码为 msgpack nil，与 Python None 一致。
var requestHandle = newRequestHandle()

func newRequestHandle() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{WriteExt: false}
	h.TypeInfos = codec.NewTypeInfos([]string{"msgpack"})
	return h
}

// responseHandle 用于解码响应——新规范：
//   - WriteExt=true：str 家族（含 str8=0xd9）按【字符串】解读；否则会被当作 []byte
//     （见 ugorji 解码 ContainerType：str 家族在 WriteExt||RawToString 为真时才是 string）；
//   - RawToString 保持 false：bin 家族仍解为 []byte，与 Python bin→bytes 一致；
//   - Raw=true：支持用 codec.Raw 两段式拆解信封；
//   - TypeInfos=["msgpack"]：读取结构体 `msgpack` tag。
var responseHandle = newResponseHandle()

func newResponseHandle() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{WriteExt: true}
	h.Raw = true
	h.TypeInfos = codec.NewTypeInfos([]string{"msgpack"})
	// 无类型 map（解入 interface{}，如 ToolSdkLoginResponse.Extra 的嵌套值）一律用 map[string]any，
	// 而非 go-codec 默认的 map[interface{}]interface{}——后者无法 json.Marshal，也不合 Python dict 语义。
	h.MapType = reflect.TypeFor[map[string]any]()
	return h
}

// marshalMsgpack 用旧规范编码请求体。
func marshalMsgpack(v any) ([]byte, error) {
	var b []byte
	if err := codec.NewEncoderBytes(&b, requestHandle).Encode(v); err != nil {
		return nil, err
	}
	return b, nil
}

// --- 响应信封解码 ---

// decodeEnvelope 将响应体解码为 header 与 data（out 为 *R）。
//
// crypted 为真：body 是 base64 文本，解码解密后按 msgpack 解析；否则按 JSON 解析。
func decodeEnvelope(body []byte, crypted bool, header *protocol.ResponseHeader, out any) error {
	if crypted {
		return decodeMsgpackEnvelope(body, header, out)
	}
	return decodeJSONEnvelope(body, header, out)
}

func decodeMsgpackEnvelope(body []byte, header *protocol.ResponseHeader, out any) error {
	mp, err := unpackCrypted(body)
	if err != nil {
		return err
	}
	var env struct {
		DataHeaders codec.Raw `msgpack:"data_headers"`
		Data        codec.Raw `msgpack:"data"`
	}
	if err := codec.NewDecoderBytes(mp, responseHandle).Decode(&env); err != nil {
		return fmt.Errorf("解析 msgpack 信封失败: %w", err)
	}
	if len(env.DataHeaders) > 0 {
		var m map[string]any
		if err := codec.NewDecoderBytes(env.DataHeaders, responseHandle).Decode(&m); err != nil {
			return fmt.Errorf("解析 data_headers 失败: %w", err)
		}
		*header = headerFromMap(m)
	}
	if len(env.Data) > 0 && out != nil {
		if err := codec.NewDecoderBytes(env.Data, responseHandle).Decode(out); err != nil {
			return fmt.Errorf("解析 data 失败: %w", err)
		}
	}
	return nil
}

func decodeJSONEnvelope(body []byte, header *protocol.ResponseHeader, out any) error {
	var env struct {
		DataHeaders json.RawMessage `json:"data_headers"`
		Data        json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("解析 JSON 信封失败: %w", err)
	}
	if len(env.DataHeaders) > 0 {
		var m map[string]any
		if err := json.Unmarshal(env.DataHeaders, &m); err != nil {
			return fmt.Errorf("解析 data_headers 失败: %w", err)
		}
		*header = headerFromMap(m)
	}
	if len(env.Data) > 0 && out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("解析 data 失败: %w", err)
		}
	}
	return nil
}

// headerFromMap 从解出的 data_headers 通用 map 中按需强转，收敛服务器对 header
// 字段类型的不稳定（已知：viewer_id 登录后为整数，short_udid 未登录时为 bool 等）。
func headerFromMap(m map[string]any) protocol.ResponseHeader {
	return protocol.ResponseHeader{
		SID:        asString(m["sid"]),
		RequestID:  asString(m["request_id"]),
		ViewerID:   asString(m["viewer_id"]),
		ServerTime: asInt64(m["servertime"]),
		ResultCode: int(asInt64(m["result_code"])),
		StoreURL:   asString(m["store_url"]),
	}
}

// asString 把任意 msgpack/JSON 标量收敛为字符串；bool 视为空串。
func asString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

// asInt64 把任意整数/浮点/数字字符串收敛为 int64。
func asInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int64:
		return x
	case uint64:
		return int64(x)
	case int:
		return int64(x)
	case float64: // JSON 数字
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		// 兜底：其余整型（int8/16/32、uint8/…）经字符串解析。
		n, _ := strconv.ParseInt(fmt.Sprint(x), 10, 64)
		return n
	}
}
