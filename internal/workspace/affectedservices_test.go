package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAffectedProjects(t *testing.T) {
	root := "/ws"
	projects := map[string]Project{
		"api":     {Path: "svc/api"},
		"sat":     {Path: "sat"}, // non-Go
		"gateway": {},            // no Path -> defaults to name "gateway"
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
		got := strings.Join(AffectedProjects(root, projects, tc.changed), ",")
		if got != tc.want {
			t.Errorf("AffectedProjects(%v) = %q, want %q", tc.changed, got, tc.want)
		}
	}

	if AffectedProjects(root, nil, []string{abs("x")}) != nil {
		t.Error("no projects should yield nil")
	}
}
