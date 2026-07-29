package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

// capture 收集日志记录，供断言级别与内容。
type capture struct {
	slog.Handler
	recs *[]slog.Record
}

func (h capture) Enabled(context.Context, slog.Level) bool { return true }
func (h capture) Handle(_ context.Context, r slog.Record) error {
	*h.recs = append(*h.recs, r)
	return nil
}

func captureLogger() (*slog.Logger, *[]slog.Record) {
	recs := &[]slog.Record{}
	return slog.New(capture{Handler: slog.Default().Handler(), recs: recs}), recs
}

func levelsOf(recs []slog.Record) []slog.Level {
	out := make([]slog.Level, len(recs))
	for i, r := range recs {
		out[i] = r.Level
	}
	return out
}

// errorResponse 是带 server_error 的响应载体（实现 protocol.ErrorCarrier）。
type errorResponse struct {
	protocol.ResponseBase
}

// serveBusinessError 起一台返回指定业务错误码的假服务器。
func serveBusinessError(t *testing.T, resultCode int, message string) *httptest.Server {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"data_headers": map[string]any{"result_code": resultCode},
		"data":         map[string]any{"server_error": map[string]any{"message": message, "status": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 业务错误默认记 Warn，不记 Error。
//
// 传输层【不知道后果】：会话失效那一类紧接着就被 sessionGuard 自愈了，记 Error 会让一次
// 成功的自愈在日志里留下一条吓人的错误；模块的业务错误则会变成该任务的 Result.Err，由调用
// 方决定它算不算失败。谁知道后果，谁记 Error。
func TestBusinessErrorLogsWarnNotError(t *testing.T) {
	srv := serveBusinessError(t, 6002, "请回到标题界面")
	logger, recs := captureLogger()
	c := New(&fakeCred{apiRoot: srv.URL}, WithHTTPClient(srv.Client()), WithLogger(logger))

	var out errorResponse
	if _, err := c.transport(context.Background(), &fakeRequest{}, &out); err == nil {
		t.Fatal("业务错误应返回错误")
	}

	for _, r := range *recs {
		if r.Level == slog.LevelError {
			t.Errorf("可自愈的业务错误不该记 Error（会话失效正是这一类）：%q", r.Message)
		}
	}
	if !hasLevel(*recs, slog.LevelWarn) {
		t.Errorf("应记一条 Warn，实际级别：%v", levelsOf(*recs))
	}
}

// 确定不可恢复的那一档（result_code 203）本层就是终点，记 Error 名副其实。
func TestUnrecoverableBusinessErrorLogsError(t *testing.T) {
	srv := serveBusinessError(t, 203, "客户端版本过低")
	logger, recs := captureLogger()
	c := New(&fakeCred{apiRoot: srv.URL}, WithHTTPClient(srv.Client()), WithLogger(logger))

	var out errorResponse
	if _, err := c.transport(context.Background(), &fakeRequest{}, &out); err == nil {
		t.Fatal("不可恢复的业务错误应返回错误")
	}

	if !hasLevel(*recs, slog.LevelError) {
		t.Errorf("不可恢复的业务错误应记 Error，实际级别：%v", levelsOf(*recs))
	}
}

func hasLevel(recs []slog.Record, want slog.Level) bool {
	for _, r := range recs {
		if r.Level == want {
			return true
		}
	}
	return false
}
