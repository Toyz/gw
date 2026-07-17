package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAffectedServices(t *testing.T) {
	root := "/ws"
	services := map[string]Service{
		"api":     {Path: "svc/api"},
		"sat":     {Path: "sat", Lang: "rust"}, // non-Go, path == would-be name too
		"gateway": {},                          // no Path -> defaults to name "gateway"
	}
	abs := func(p string) string { return filepath.FromSlash(filepath.Join(root, p)) }

	cases := []struct {
		changed []string
		want    string // comma-joined sorted
	}{
		{[]string{abs("svc/api/main.go")}, "api"},
		{[]string{abs("sat/src/lib.rs")}, "sat"},
		{[]string{abs("gateway/x.go")}, "gateway"},
		{[]string{abs("svc/api/a.go"), abs("sat/Cargo.toml")}, "api,sat"},
		{[]string{abs("docs/readme.md")}, ""}, // owned by nothing
		{[]string{abs("satellite/x")}, ""},    // must not match "sat" by prefix
	}
	for _, tc := range cases {
		got := strings.Join(AffectedServices(root, services, tc.changed), ",")
		if got != tc.want {
			t.Errorf("AffectedServices(%v) = %q, want %q", tc.changed, got, tc.want)
		}
	}

	if AffectedServices(root, nil, []string{abs("x")}) != nil {
		t.Error("no services should yield nil")
	}
}
