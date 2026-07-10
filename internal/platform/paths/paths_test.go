package paths

import "testing"

func TestDefault(t *testing.T) {
	p := Default()
	if p.Cache != "cache" {
		t.Fatalf("Cache = %q, want %q", p.Cache, "cache")
	}
	if p.Data != "data" {
		t.Fatalf("Data = %q, want %q", p.Data, "data")
	}
}
