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

func TestGraphTopoOrder(t *testing.T) {
	// core <- api <- gateway ; core <- worker ; unrelated stands alone.
	mods := []Module{
		mod("example.com/core"),
		mod("example.com/api", "example.com/core"),
		mod("example.com/gateway", "example.com/api"),
		mod("example.com/worker", "example.com/core"),
		mod("example.com/unrelated"),
	}
	g := BuildGraph(mods)
	order := g.TopoOrder()

	if len(order) != len(mods) {
		t.Fatalf("TopoOrder returned %d modules, want %d: %v", len(order), len(mods), order)
	}
	pos := map[string]int{}
	for i, p := range order {
		pos[p] = i
	}
	// The release contract: a module must never appear before something it
	// depends on (you tag the dependency first).
	for _, m := range mods {
		for _, dep := range g.Dependencies(m.Path) {
			if pos[dep] > pos[m.Path] {
				t.Fatalf("%s (pos %d) ordered before its dependency %s (pos %d): %v",
					m.Path, pos[m.Path], dep, pos[dep], order)
			}
		}
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
