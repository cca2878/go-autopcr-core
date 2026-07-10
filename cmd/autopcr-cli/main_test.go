package main

import "testing"

func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"version", []string{"version"}, 0},
		{"version short flag", []string{"-v"}, 0},
		{"help", []string{"help"}, 0},
		{"no args", nil, 2},
		{"unknown command", []string{"bogus"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != tc.want {
				t.Fatalf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}
