package workspace

import "testing"

func TestCompareVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign of compareVer(a,b)
	}{
		// toolchain values: double-digit minor must sort above single-digit.
		{"go1.10.0", "go1.9.0", 1},
		{"go1.9.0", "go1.10.0", -1},
		{"go1.26.0", "go1.26.0", 0},
		// bare go directive.
		{"1.25.0", "1.9.0", 1},
		// dependency semver.
		{"v0.9.1", "v0.8.0", 1},
		{"v0.10.0", "v0.9.0", 1},
	}
	for _, tc := range cases {
		got := compareVer(tc.a, tc.b)
		if (got > 0) != (tc.want > 0) || (got < 0) != (tc.want < 0) {
			t.Errorf("compareVer(%q,%q)=%d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSortedVersionsToolchain(t *testing.T) {
	m := Mismatch{Dep: ToolchainDirective, Versions: map[string][]string{
		"go1.9.0":  {"a"},
		"go1.10.0": {"b"},
	}}
	got := m.SortedVersions()
	if got[0] != "go1.10.0" {
		t.Fatalf("highest toolchain should be go1.10.0, got %v", got)
	}
}
