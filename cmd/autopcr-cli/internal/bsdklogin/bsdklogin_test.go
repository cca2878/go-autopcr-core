package bsdklogin

import (
	"context"
	"testing"
	"time"
)

// TestLoginEmptyCredentials 验证空账号/密码在发起任何网络请求前就快速失败。
// （空值校验位于 bsdkv3.NewClient 调用之前，故本用例不触网。）
func TestLoginEmptyCredentials(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cases := []struct{ user, pass string }{
		{"", ""},
		{"", "pass"},
		{"user", ""},
	}
	for _, c := range cases {
		if _, _, err := Login(ctx, c.user, c.pass); err == nil {
			t.Errorf("Login(%q,%q) 期望返回错误，实际为 nil", c.user, c.pass)
		}
	}
}
