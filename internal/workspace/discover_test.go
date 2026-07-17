package workspace

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, rel string
		want         bool
	}{
		// "**" matches whole segments, not substrings.
		{"**/testdata", "testdata", true},
		{"**/testdata", "apps/testdata", true},
		{"**/testdata", "a/b/testdata", true},
		{"**/testdata", "testdata-fixtures", false}, // the bug: must not match a prefix
		{"**/testdata", "apps/testdata2", false},
		{"**/testdata", "testdatax/mod", false},
		// trailing "**" matches the dir itself and anything under it.
		{"examples/**", "examples", true},
		{"examples/**", "examples/demo", true},
		{"examples/**", "examples/a/b", true},
		{"examples/**", "example", false},
		{"examples/**", "other/examples", false},
		// slashless pattern matches any single segment (any depth).
		{"vendor", "vendor", true},
		{"vendor", "a/vendor", true},
		{"vendor", "a/vendorx", false},
		// wildcards within a segment.
		{"*.gen", "a/b.gen", true},
		{"testdata", "pkg/testdata", true},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.rel); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.rel, got, tc.want)
		}
	}
}
