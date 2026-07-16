package labyrinth

import "testing"

// TestCleanGuildName 检查公会名里的换行标记被抹平——母数据存的是【字面量】反斜杠 n
// （游戏内用于折行，如 `破晓\n之星`），直接显示会露出转义符。
func TestCleanGuildName(t *testing.T) {
	cases := map[string]string{
		`破晓\n之星`:             "破晓 之星",
		`王宫骑士团\n（NIGHTMARE）`: "王宫骑士团 （NIGHTMARE）",
		"美食殿堂":               "美食殿堂",
		"拉比林斯\n":             "拉比林斯",
		`  咲恋救济院 `:           "咲恋救济院",
	}
	for in, want := range cases {
		if got := cleanGuildName(in); got != want {
			t.Errorf("cleanGuildName(%q) = %q, want %q", in, got, want)
		}
	}
}
