// Package logging 基于标准库 log/slog 提供统一日志构建。
package logging

import (
	"log/slog"
	"os"
)

// New 依据 debug 标志创建一个写往 stderr 的文本 slog.Logger。
func New(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// Setup 创建 logger、设为 slog 全局默认，并返回该 logger。
func Setup(debug bool) *slog.Logger {
	logger := New(debug)
	slog.SetDefault(logger)
	return logger
}
