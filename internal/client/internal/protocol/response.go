package protocol

// ResponseHeader 是响应信封中的 data_headers。
//
// 注：服务器对部分字段的类型并不稳定（例如 short_udid 在未登录时会返回 bool
// false 而非字符串）。这里只声明确实需要的字段，未知键由解码器忽略，避免类型冲突。
type ResponseHeader struct {
	SID        string `msgpack:"sid" json:"sid"`
	RequestID  string `msgpack:"request_id" json:"request_id"`
	ViewerID   string `msgpack:"viewer_id" json:"viewer_id"`
	ServerTime int64  `msgpack:"servertime" json:"servertime"`
	ResultCode int    `msgpack:"result_code" json:"result_code"`
	// StoreURL 出现在维护状态响应头中，用于版本检测（M1 仅捕获，不处理）。
	StoreURL string `msgpack:"store_url" json:"store_url"`
}

// ErrorInfo 是响应数据中的 server_error。
type ErrorInfo struct {
	Title   string `msgpack:"title" json:"title"`
	Message string `msgpack:"message" json:"message"`
	Status  int    `msgpack:"status" json:"status"`
}

// ResponseBase 供所有响应内嵌，携带 server_error。
type ResponseBase struct {
	ServerError *ErrorInfo `msgpack:"server_error" json:"server_error"`
}

// UserGold 是金币信息（付费/免费两部分）。服务端在几十种响应里都会回传 user_gold 作为余额快照，
// 故放在协议基包而非某个域包，供各域按需内嵌。
type UserGold struct {
	GoldIDPay  int64 `msgpack:"gold_id_pay" json:"gold_id_pay"`
	GoldIDFree int64 `msgpack:"gold_id_free" json:"gold_id_free"`
}

// Total 返回付费+免费金币合计（对应 ref get_inventory 的 mana 分支）。
func (g *UserGold) Total() int64 {
	if g == nil {
		return 0
	}
	return g.GoldIDPay + g.GoldIDFree
}

// GetServerError 实现 ErrorCarrier 接口。
func (r *ResponseBase) GetServerError() *ErrorInfo { return r.ServerError }

// ErrorCarrier 让 transport 能在不知具体响应类型的情况下取出 server_error。
type ErrorCarrier interface {
	GetServerError() *ErrorInfo
}
