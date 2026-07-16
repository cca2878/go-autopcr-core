package transport

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
	"github.com/ugorji/go/codec"
)

func marshalWith(t *testing.T, h *codec.MsgpackHandle, v any) []byte {
	t.Helper()
	var b []byte
	if err := codec.NewEncoderBytes(&b, h).Encode(v); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestMarshalMsgpackNoStr8 验证长字符串走 raw16(0xda) 而非 str8(0xd9)，
// 即复刻 use_bin_type=False 的兼容语义。
func TestMarshalMsgpackNoStr8(t *testing.T) {
	b, err := marshalMsgpack(struct {
		S string `msgpack:"s"`
	}{S: string(bytes.Repeat([]byte("x"), 40))})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.IndexByte(b, 0xd9) >= 0 {
		t.Fatalf("编码中出现 str8 标记 0xd9：%x", b)
	}
	if bytes.IndexByte(b, 0xda) < 0 {
		t.Fatalf("编码中缺少 raw16 标记 0xda：%x", b)
	}
}

// TestMarshalMsgpackNilPointer 验证 nil 指针字段编码为 msgpack nil 且字段名保留。
func TestMarshalMsgpackNilPointer(t *testing.T) {
	b, err := marshalMsgpack(struct {
		P *string `msgpack:"p"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.IndexByte(b, 0xc0) < 0 {
		t.Fatalf("nil 指针未编码为 msgpack nil(0xc0)：%x", b)
	}
	if !bytes.Contains(b, []byte("p")) {
		t.Fatalf("nil 指针字段名被省略：%x", b)
	}
}

// TestMarshalMsgpackRequestRoundTrip 校验内嵌字段(viewer_id)、普通字段与 nil 指针字段。
func TestMarshalMsgpackRequestRoundTrip(t *testing.T) {
	req := &sdk.ToolSdkLoginRequest{
		UID:       "12345",
		AccessKey: "some-access-key-value-longer-than-31-bytes",
		Platform:  "2",
		ChannelID: "1",
	}
	req.SetViewerID("0")

	b, err := marshalMsgpack(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("viewer_id")) {
		t.Fatalf("内嵌 viewer_id 未扁平化：%x", b)
	}
	var got sdk.ToolSdkLoginRequest
	if err := codec.NewDecoderBytes(b, responseHandle).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.UID != "12345" || got.AccessKey != req.AccessKey || got.Platform != "2" || got.ChannelID != "1" {
		t.Fatalf("字段不匹配: %+v", got)
	}
	if got.ViewerID != "0" {
		t.Fatalf("viewer_id=%q want 0", got.ViewerID)
	}
	if got.Challenge != nil {
		t.Fatalf("challenge 应为 nil，实际 %v", *got.Challenge)
	}
}

// TestDecodeEnvelopeMsgpack 用「新协议(str8)」服务器句柄编码响应，
// 验证信封解码对 str8/bin 的健壮性以及部分 DTO 解码。
func TestDecodeEnvelopeMsgpack(t *testing.T) {
	// 服务器侧：故意用 WriteExt=true（新协议，长字符串走 str8）编码。
	serverHandle := &codec.MsgpackHandle{WriteExt: true}
	env := map[string]any{
		"data_headers": map[string]any{
			"servertime":  int64(1700000000),
			"viewer_id":   "42",
			"result_code": 1,
			"sid":         "s",
		},
		"data": map[string]any{
			"user_info": map[string]any{
				"user_name":    "骑士君（一个足够长的昵称占位以触发长字符串编码路径）",
				"team_level":   123,
				"user_stamina": 50,
			},
			"daily_reset_time": int64(1700000123),
		},
	}
	mp := marshalWith(t, serverHandle, env)
	enc, err := encrypt(mp, testKey)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(enc)

	var header protocol.ResponseHeader
	var out account.LoadIndexResponse
	if err := decodeEnvelope([]byte(b64), true, &header, &out); err != nil {
		t.Fatal(err)
	}
	if header.ViewerID != "42" || header.ServerTime != 1700000000 {
		t.Fatalf("header 解析错误: %+v", header)
	}
	if out.UserInfo == nil || out.UserInfo.TeamLevel != 123 || out.UserInfo.UserStamina != 50 {
		t.Fatalf("user_info 解析错误: %+v", out.UserInfo)
	}
	if out.DailyResetTime != 1700000123 {
		t.Fatalf("daily_reset_time=%d", out.DailyResetTime)
	}
}

// TestDecodeEnvelopeIntViewerID 锁定：登录成功后 data_headers.viewer_id 是整数
// （非字符串），应被收敛为字符串而非解码失败。
func TestDecodeEnvelopeIntViewerID(t *testing.T) {
	serverHandle := &codec.MsgpackHandle{WriteExt: true}
	env := map[string]any{
		"data_headers": map[string]any{
			"viewer_id":   uint64(123456789), // 登录后为整数
			"servertime":  int64(1700000000),
			"short_udid":  false, // 已知怪癖：bool
			"result_code": 1,
		},
		"data": map[string]any{},
	}
	mp := marshalWith(t, serverHandle, env)
	enc, err := encrypt(mp, testKey)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(enc)

	var header protocol.ResponseHeader
	var out account.LoadIndexResponse
	if err := decodeEnvelope([]byte(b64), true, &header, &out); err != nil {
		t.Fatal(err)
	}
	if header.ViewerID != "123456789" {
		t.Fatalf("viewer_id=%q want 123456789", header.ViewerID)
	}
	if header.ServerTime != 1700000000 {
		t.Fatalf("servertime=%d", header.ServerTime)
	}
}

// TestResponseDecodeStrAsString 锁定「响应用新规范」：str 家族解入 interface{}
// 时，responseHandle 应得 string；作为对照，旧规范的 requestHandle 会得 []byte。
func TestResponseDecodeStrAsString(t *testing.T) {
	// 服务器用新规范编码一个含字符串值的 map。
	serverHandle := &codec.MsgpackHandle{WriteExt: true}
	payload := marshalWith(t, serverHandle, map[string]any{"k": "一个足够长的字符串值 str family into interface"})

	var m map[string]any
	if err := codec.NewDecoderBytes(payload, responseHandle).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["k"].(string); !ok {
		t.Fatalf("responseHandle 应把 str 解为 string，实际 %T", m["k"])
	}

	// 对照：旧规范 handle 会把同样的 str 解为 []byte。
	var m2 map[string]any
	if err := codec.NewDecoderBytes(payload, requestHandle).Decode(&m2); err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["k"].([]byte); !ok {
		t.Fatalf("对照：requestHandle 应把 str 解为 []byte，实际 %T", m2["k"])
	}
}

// TestDecodeEnvelopeJSONServerError 验证明文 JSON 路径与 server_error 提取。
func TestDecodeEnvelopeJSONServerError(t *testing.T) {
	body := `{"data_headers":{"result_code":203},"data":{"server_error":{"title":"t","message":"维护中","status":1}}}`
	var header protocol.ResponseHeader
	var out account.LoadIndexResponse
	if err := decodeEnvelope([]byte(body), false, &header, &out); err != nil {
		t.Fatal(err)
	}
	if header.ResultCode != 203 {
		t.Fatalf("result_code=%d", header.ResultCode)
	}
	se := out.GetServerError()
	if se == nil || se.Message != "维护中" {
		t.Fatalf("server_error 解析错误: %+v", se)
	}
}

// TestDecodeEnvelopeRiskExtra 锁定风控数据采集：tool/sdk_login 响应 data 里除已声明的 is_risk
// 外的所有【未声明】字段，应经 codec.MissingFielder 原样落入 ToolSdkLoginResponse.Extra（而非被
// 丢弃），以便看清 is_risk 到底带了什么。
func TestDecodeEnvelopeRiskExtra(t *testing.T) {
	serverHandle := &codec.MsgpackHandle{WriteExt: true}
	env := map[string]any{
		"data_headers": map[string]any{"result_code": 1, "servertime": int64(1700000000)},
		"data": map[string]any{
			"is_risk":   true,
			"risk_type": "一个足够长的未知字段值以触发长字符串编码路径", // str 家族
			"threshold": int64(42),
			"detail":    map[string]any{"code": int64(9)}, // 嵌套 map 也应保留
		},
	}
	mp := marshalWith(t, serverHandle, env)
	enc, err := encrypt(mp, testKey)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(enc)

	var header protocol.ResponseHeader
	var out sdk.ToolSdkLoginResponse
	if err := decodeEnvelope([]byte(b64), true, &header, &out); err != nil {
		t.Fatal(err)
	}
	if !out.IsRisk {
		t.Fatal("is_risk 应解为 true")
	}
	if out.Extra == nil {
		t.Fatal("未声明字段应落入 Extra，实际为 nil")
	}
	if _, ok := out.Extra["is_risk"]; ok {
		t.Error("已声明字段 is_risk 不应出现在 Extra 中")
	}
	if v, ok := out.Extra["risk_type"].(string); !ok || v == "" {
		t.Errorf("Extra[risk_type]=%v，期望非空字符串", out.Extra["risk_type"])
	}
	if v := asInt64(out.Extra["threshold"]); v != 42 {
		t.Errorf("Extra[threshold]=%v，期望 42", out.Extra["threshold"])
	}
	if _, ok := out.Extra["detail"].(map[string]any); !ok {
		t.Errorf("Extra[detail] 应保留为嵌套 map，实际 %T", out.Extra["detail"])
	}
}
