package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func mod(path string, requires ...string) Module {
	r := map[string]string{}
	for _, dep := range requires {
		r[dep] = "v0.0.0"
	}
	return Module{Path: path, Dir: "/ws/" + filepath.Base(path), Requires: r}
}

func TestGraphTransitiveDependents(t *testing.T) {
	// core <- api <- gateway ; core <- worker
	mods := []Module{
		mod("example.com/core"),
		mod("example.com/api", "example.com/core"),
		mod("example.com/gateway", "example.com/api"),
		mod("example.com/worker", "example.com/core"),
		mod("example.com/unrelated"),
	}
	g := BuildGraph(mods)

	got := g.TransitiveDependents([]string{"example.com/core"})
	want := []string{"example.com/api", "example.com/core", "example.com/gateway", "example.com/worker"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("transitive dependents of core = %v, want %v", got, want)
	}

	// changing a leaf only affects itself
	got = g.TransitiveDependents([]string{"example.com/gateway"})
	if strings.Join(got, ",") != "example.com/gateway" {
		t.Fatalf("gateway should only affect itself, got %v", got)
	}

	if deps := g.Dependencies("example.com/api"); strings.Join(deps, ",") != "example.com/core" {
		t.Fatalf("api deps = %v", deps)
	}
}

func TestOwningModule(t *testing.T) {
	mods := []Module{
		{Path: "a", Dir: "/ws/a"},
		{Path: "a-nested", Dir: "/ws/a/nested"}, // deeper module wins
		{Path: "b", Dir: "/ws/b"},
	}
	cases := map[string]string{
		"/ws/a/main.go":         "a",
		"/ws/a/nested/thing.go": "a-nested",
		"/ws/b/x/y.go":          "b",
		"/ws/outside/z.go":      "",
		"/ws/ab/z.go":           "", // must not match "a" by raw prefix
	}
	for file, want := range cases {
		m, ok := OwningModule(mods, filepath.FromSlash(file))
		got := ""
		if ok {
			got = m.Path
		}
		if got != want {
			t.Fatalf("OwningModule(%s) = %q, want %q", file, got, want)
		}
	}
}
