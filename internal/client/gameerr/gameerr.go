// Package gameerr 定义跨层共享的错误类型。
//
// 对应原 Python 项目中的 PanicError（model/error.py）与 apiclient 的
// NetworkException / ApiException。模块层的 SkipError/AbortError 属自动化
// 控制流，将在 M3 引入。
package gameerr

import (
	"errors"
	"fmt"
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
// 当前 tool/sdk_login 响应只暴露 is_risk 布尔、不含 challenge/gt 等票据；将来该场景频发、
// 需按真实字段设计求解时，应扩展 sdk.ToolSdkLoginResponse 采集服务器 payload 并在此携带，
// 以便积累数据（见架构决策：captcha 移出核心，is_risk 暂硬失败但需响亮可诊断）。
type RiskError struct {
	Attempts int   // 已完成的「求解→重登」尝试轮数（0 表示求解阶段即失败、未发出重登）
	Cause    error // 最近一次失败成因（无求解器时 Unwrap 命中 captcha.ErrNoSolver）
}

func (e *RiskError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("帐号触发风控(is_risk)未通过：%v（已尝试 %d 轮）", e.Cause, e.Attempts)
	}
	return fmt.Sprintf("帐号触发风控(is_risk)，%d 轮验证码后仍未通过", e.Attempts)
}

func (e *RiskError) Unwrap() error { return e.Cause }

// Risk 构造一个 RiskError。
func Risk(attempts int, cause error) *RiskError { return &RiskError{Attempts: attempts, Cause: cause} }

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

// AsPanic 在 err 为 PanicError 时返回它。
func AsPanic(err error) (*PanicError, bool) {
	var p *PanicError
	if errors.As(err, &p) {
		return p, true
	}
	return nil, false
}
