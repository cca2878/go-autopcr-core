package client

import "testing"

// TestEmbeddedRainbow 验证内嵌 rainbow 能被解析且非空（不触网）。
func TestEmbeddedRainbow(t *testing.T) {
	rb, err := defaultRainbow()
	if err != nil {
		t.Fatalf("解析内嵌 rainbow: %v", err)
	}
	if len(rb) == 0 {
		t.Fatal("内嵌 rainbow 为空")
	}
}
