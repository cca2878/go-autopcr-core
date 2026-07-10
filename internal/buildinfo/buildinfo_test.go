package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetPopulatesRuntimeFields(t *testing.T) {
	info := Get()
	if info.GoVersion != runtime.Version() {
		t.Fatalf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; info.Platform != want {
		t.Fatalf("Platform = %q, want %q", info.Platform, want)
	}
}

func TestStringContainsVersion(t *testing.T) {
	info := Get()
	if !strings.Contains(info.String(), info.Version) {
		t.Fatalf("String() = %q, does not contain Version %q", info.String(), info.Version)
	}
}
