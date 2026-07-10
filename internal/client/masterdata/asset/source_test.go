package asset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cca2878/go-autopcr/internal/client/internal/urlx"
)

// TestFetchMasterdata 用本地服务器模拟层级清单 + pool，验证解析与下载。
func TestFetchMasterdata(t *testing.T) {
	const ver = 20240101
	const md5 = "ab12cd34"
	assetBytes := []byte("fake-unity3d-bytes")

	mux := http.NewServeMux()
	base := "/Manifest/AssetBundles/Android/20240101/"
	// 顶层清单：引用一个子清单。
	mux.HandleFunc(base+"manifest/manifest_assetmanifest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("manifest/masterdata_assetmanifest,mmm,manifest,100\n"))
	})
	// 子清单：列出 masterdata 资源。
	mux.HandleFunc(base+"manifest/masterdata_assetmanifest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a/masterdata_master.unity3d," + md5 + ",mdb,999\n"))
	})
	// pool 资源。
	mux.HandleFunc("/pool/AssetBundles/Android/ab/"+md5, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(assetBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewSource(WithRes(urlx.MustParseBase(srv.URL)))
	got, err := s.FetchMasterdata(context.Background(), ver)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(assetBytes) {
		t.Fatalf("下载内容不符: %q", got)
	}
}

func TestFetchMasterdataMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "manifest_assetmanifest") {
			_, _ = w.Write([]byte("a/other.unity3d,xx,t,1\n")) // 没有 masterdata
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewSource(WithRes(urlx.MustParseBase(srv.URL)))
	if _, err := s.FetchMasterdata(context.Background(), 1); err == nil {
		t.Fatal("清单中无 masterdata 应报错")
	}
}
