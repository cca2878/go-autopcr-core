package asset

import "testing"

func TestParseLine(t *testing.T) {
	cases := []struct {
		name                       string
		line                       string
		ok                         bool
		wantURL, wantMD5, wantType string
	}{
		{"asset", "a/x.unity3d,md5abc,ab,123", true, "a/x.unity3d", "md5abc", "ab"},
		{"manifest", "manifest/sub,mmm,manifest,10", true, "manifest/sub", "mmm", "manifest"},
		{"offset", "u,m,X,typeval,size,extra", true, "u", "m", "typeval"}, // >5 列 → type=splits[3]
		{"short", "bad,line", false, "", "", ""},
		{"empty", "", false, "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseLine(c.line, "cat")
			if ok != c.ok {
				t.Fatalf("ok=%v want %v", ok, c.ok)
			}
			if !ok {
				return
			}
			if got.URL != c.wantURL || got.MD5 != c.wantMD5 || got.Type != c.wantType {
				t.Fatalf("got %+v", got)
			}
			if got.Category != "cat" {
				t.Fatalf("category=%q", got.Category)
			}
		})
	}
}
