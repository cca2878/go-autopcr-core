package asset

import (
	"errors"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

// 同一个 HTTPError 落在哪一类由状态码决定——这正是「多主机故障转移」要的判据：
// 5xx/429 换台主机还有戏，404 换到哪台都一样。
func TestHTTPErrorKindByStatus(t *testing.T) {
	cases := []struct {
		status    int
		retryable bool
		want      errs.Kind
	}{
		{500, true, errs.KindTransient},
		{502, true, errs.KindTransient},
		{503, true, errs.KindTransient},
		{429, true, errs.KindTransient},
		{404, false, errs.KindRejected},
		{403, false, errs.KindRejected},
		{400, false, errs.KindRejected},
	}
	for _, c := range cases {
		e := &HTTPError{URL: "https://cdn.example/pool/a/bc/bcdef", Status: c.status}
		if got := e.Retryable(); got != c.retryable {
			t.Errorf("状态 %d：Retryable = %v, want %v", c.status, got, c.retryable)
		}
		if got := errs.Classify(e).Kind; got != c.want {
			t.Errorf("状态 %d：Classify = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestHTTPErrorMessageCarriesURLAndStatus(t *testing.T) {
	e := &HTTPError{URL: "https://cdn.example/x", Status: 503}
	if msg := e.Error(); msg != "GET https://cdn.example/x: 状态 503" {
		t.Errorf("Error() = %q", msg)
	}
}

// 「清单里没有这个资源」不是损坏也不是暂时故障：换台主机重来同样没有，故归 Rejected。
func TestManifestSentinelKinds(t *testing.T) {
	if got := errs.Classify(ErrNotInManifest).Kind; got != errs.KindRejected {
		t.Errorf("ErrNotInManifest 分类 = %v, want KindRejected", got)
	}
	if got := errs.Classify(ErrBadManifest).Kind; got != errs.KindCorrupt {
		t.Errorf("ErrBadManifest 分类 = %v, want KindCorrupt", got)
	}
}

func TestDownloadRejectsShortMD5(t *testing.T) {
	s := NewSource()
	_, err := s.Download(t.Context(), &Content{URL: "a/x.unity3d", MD5: "z"})
	if !errors.Is(err, ErrBadManifest) {
		t.Fatalf("md5 键过短应命中 ErrBadManifest，得到 %v", err)
	}
}
