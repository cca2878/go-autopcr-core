package urlx

import (
	"net/url"
	"testing"
)

func TestParseBaseTrailingSlash(t *testing.T) {
	cases := []struct {
		raw      string
		wantPath string
	}{
		{"https://h/client_ob_771", "/client_ob_771/"},  // 补尾斜杠
		{"https://h/client_ob_771/", "/client_ob_771/"}, // 已带则不变
		{"https://h", "/"}, // host-only → "/"
		{"https://h/", "/"},
	}
	for _, c := range cases {
		u, err := ParseBase(c.raw)
		if err != nil {
			t.Fatalf("ParseBase(%q): %v", c.raw, err)
		}
		if u.Path != c.wantPath {
			t.Fatalf("ParseBase(%q).Path = %q, want %q", c.raw, u.Path, c.wantPath)
		}
	}
}

// TestResolveReferenceKeepsBaseSegment 锁死修复：base 带尾斜杠后，ResolveReference
// 相对引用不会丢掉 base 路径最后一段（RFC 3986 的坑）。
func TestResolveReferenceKeepsBaseSegment(t *testing.T) {
	base := MustParseBase("https://h/client_ob_771")
	got := base.ResolveReference(&url.URL{Path: "Manifest/AssetBundles/Android/42/"}).String()
	want := "https://h/client_ob_771/Manifest/AssetBundles/Android/42/"
	if got != want {
		t.Fatalf("ResolveReference = %q, want %q", got, want)
	}

	// 带 query 的端点也应正确保留（游戏 API 场景）。
	ref, _ := url.Parse("source_ini/index?format=json")
	got = base.ResolveReference(ref).String()
	want = "https://h/client_ob_771/source_ini/index?format=json"
	if got != want {
		t.Fatalf("ResolveReference(query) = %q, want %q", got, want)
	}
}
