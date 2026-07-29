package discovery

import "testing"

// 保留【全部】下发主机是故障转移的前提：服务端下发多台正是让它们互为备份（实测 l1/l3/l4），
// 只取首个等于把冗余丢掉，首台一挂就只能回退到写死的内置 CDN。
func TestResolveResURLsKeepsEveryHost(t *testing.T) {
	got := ResolveResURLs(0, []string{
		"l1-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
		"l3-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
		"l4-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
	})

	if len(got) != 3 {
		t.Fatalf("解析出 %d 台, want 3", len(got))
	}
	want := []string{
		"https://l1-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
		"https://l3-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
		"https://l4-prod-patch-gzlj.bilibiligame.net/client_ob_771/",
	}
	for i, u := range got {
		if u.String() != want[i] {
			t.Errorf("[%d] = %q, want %q", i, u, want[i])
		}
	}
}

// scheme 由 res_http_type 决定（实测 0=https）。
func TestResolveResURLsScheme(t *testing.T) {
	if got := ResolveResURLs(0, []string{"h/x"}); got[0].Scheme != "https" {
		t.Errorf("res_http_type=0 应为 https，得到 %q", got[0].Scheme)
	}
	if got := ResolveResURLs(1, []string{"h/x"}); got[0].Scheme != "http" {
		t.Errorf("res_http_type=1 应为 http，得到 %q", got[0].Scheme)
	}
}

// 坏条目跳过而非整体作废：还剩一台能用就不该退回内置默认 CDN。
func TestResolveResURLsSkipsBadEntries(t *testing.T) {
	got := ResolveResURLs(0, []string{"", "  \t ", "good-host.example/path/"})
	if len(got) != 1 {
		t.Fatalf("解析出 %d 台, want 1", len(got))
	}
	if got[0].Host != "good-host.example" {
		t.Errorf("Host = %q", got[0].Host)
	}
}

// 下发为空时返回空切片（不是 nil 元素），由上层回退内置默认 CDN。
func TestResolveResURLsEmpty(t *testing.T) {
	if got := ResolveResURLs(0, nil); len(got) != 0 {
		t.Errorf("空下发应得空结果，得到 %v", got)
	}
}
