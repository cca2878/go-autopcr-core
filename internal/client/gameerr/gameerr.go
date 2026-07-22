// Package gameerr 定义跨层共享的错误类型。
//
// 对应原 Python 项目中的 PanicError（model/error.py）与 apiclient 的
// NetworkException / ApiException。模块层的 SkipError/AbortError 属自动化
// 控制流，将在 M3 引入。
package gameerr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// PanicError 表示致命错误：应中止整条流程（对应原项目 PanicError）。
type PanicError struct {
	Msg string
}

func (e *PanicError) Error() string { return e.Msg }

// Panic 构造一个 PanicError。
func Panic(format string, a ...any) *PanicError {
	return &PanicError{Msg: fmt.Sprintf(format, a...)}
}

// RiskError 表示游戏服登录触发风控(is_risk)但未能通过验证码解除。
//
// 这是一个 distinct、可诊断的错误（区别于泛化 PanicError）：外壳可用 errors.As 命中它，
// 用 errors.Is 命中其成因（如 captcha.ErrNoSolver 表示未注入求解器）。
//
// Payload 携带触发风控的服务器响应里【未建模的原始字段】快照（见 sdk.ToolSdkLoginResponse.Extra）：
// tool/sdk_login 已声明字段只有 is_risk，故此处收纳的是其余未知键，用于积累数据、便于将来按真实
// 字段设计求解（见架构决策：captcha 移出核心，is_risk 暂硬失败但需响亮可诊断）。可能为空。
type RiskError struct {
	Attempts int            // 已完成的「求解→重登」尝试轮数（0 表示求解阶段即失败、未发出重登）
	Cause    error          // 最近一次失败成因（无求解器时 Unwrap 命中 captcha.ErrNoSolver）
	Payload  map[string]any // 触发风控的响应中未建模字段的快照（可能为空）
}

func (e *RiskError) Error() string {
	var msg string
	if e.Cause != nil {
		msg = fmt.Sprintf("帐号触发风控(is_risk)未通过：%v（已尝试 %d 轮）", e.Cause, e.Attempts)
	} else {
		msg = fmt.Sprintf("帐号触发风控(is_risk)，%d 轮验证码后仍未通过", e.Attempts)
	}
	// 有未知载荷时附上其 JSON，使其在任何仅取错误字符串的场景（CLI 日志、gomobile 异常消息）都可见。
	if len(e.Payload) > 0 {
		if js, err := json.Marshal(e.Payload); err == nil {
			msg += fmt.Sprintf("；风控响应载荷=%s", js)
		}
	}
	return msg
}

func (e *RiskError) Unwrap() error { return e.Cause }

// Risk 构造一个 RiskError。payload 为触发风控响应的未建模字段快照，可为 nil。
func Risk(attempts int, cause error, payload map[string]any) *RiskError {
	return &RiskError{Attempts: attempts, Cause: cause, Payload: payload}
}

// NetworkError 表示网络层失败（连接错误、超时、非 200、解码失败）。
type NetworkError struct {
	Err error
}

func (e *NetworkError) Error() string {
	if e.Err == nil {
		return "network error"
	}
	return "network error: " + e.Err.Error()
}

func (e *NetworkError) Unwrap() error { return e.Err }

// Network 构造一个 NetworkError。
func Network(err error) *NetworkError { return &NetworkError{Err: err} }

// APIError 表示游戏服务器返回的业务错误（响应中的 server_error）。
type APIError struct {
	Message    string
	Status     int
	ResultCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error (result_code=%d, status=%d): %s", e.ResultCode, e.Status, e.Message)
}

// resultCodeUnrecoverable 是服务端表示「本次请求不可恢复」的业务码（原项目对 203 的处理）。
const resultCodeUnrecoverable = 203

// maintenanceMarker 是维护中错误消息的特征串（原项目按消息文本判定维护）。
const maintenanceMarker = "维护"

// IsFatalBusiness 报告一个业务错误是否应升级为 PanicError（＝重试与重登都救不了，须直接终止）。
//
// 判据集中在此：这类「哪些业务码/消息算严重」的知识属于游戏错误词汇表，散在传输层与中间件里
// 就会出现改一处漏两处。会话失效那一类的判据不在这里——它描述的是会话生命周期而非终止性，
// 归 client 的 classifySession 所有（见 relogin.go）。
func IsFatalBusiness(resultCode int, message string) bool {
	return resultCode == resultCodeUnrecoverable || strings.Contains(message, maintenanceMarker)
}

// SessionBreakError 表示请求在【会话失效】处被打断：服务端丢弃了会话（顶号、数据不一致等），
// 客户端已自愈（重新登录），但这次请求所在的那段逻辑跨越了一次世界断点。
//
// 它存在的理由是自愈救不了调用方的推理：模块的中间结论存在 Go 局部变量里（「刚查到礼物箱有 3
// 件」「刚拿到的 enter_id」），断点之后这些快照可能已与线上不符，而框架看不见也修不了它们。
// 故对不能容忍断点的模块，这里【当场失败】而不是悄悄重发——让它在断点处 unwind，好过带着旧
// 世界的结论继续往下写。是否容忍由模块自己声明（见 automation 的 SessionAware）。
type SessionBreakError struct {
	Cause error // 触发断点的原始错误（通常是 *APIError）
}

func (e *SessionBreakError) Error() string {
	return fmt.Sprintf("会话在执行期间失效并已重新登录：本任务结果不可信、可能已部分执行，"+
		"请复查后重跑（成因: %v）", e.Cause)
}

func (e *SessionBreakError) Unwrap() error { return e.Cause }

// AsSessionBreak 在 err 为 SessionBreakError 时返回它。
func AsSessionBreak(err error) (*SessionBreakError, bool) {
	var b *SessionBreakError
	if errors.As(err, &b) {
		return b, true
	}
	return nil, false
}

// AsPanic 在 err 为 PanicError 时返回它。
func AsPanic(err error) (*PanicError, bool) {
	var p *PanicError
	if errors.As(err, &p) {
		return p, true
	}
	return nil, false
}
