package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMod writes a go.mod at root/rel/go.mod with the given body.
func writeMod(t *testing.T, root, rel, body string) string {
	t.Helper()
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	writeMod(t, root, "svc-a", "module example.com/svc-a\n\ngo 1.25.0\n")
	writeMod(t, root, "svc-b", "module example.com/svc-b\n\ngo 1.25.0\n")
	writeMod(t, root, "tools/gen", "module example.com/tools/gen\n\ngo 1.25.0\n")
	// Ignored trees.
	writeMod(t, root, "vendor/x", "module example.com/vendor/x\n\ngo 1.25.0\n")
	writeMod(t, root, "examples/demo", "module example.com/examples/demo\n\ngo 1.25.0\n")

	cfg := Config{Ignore: []string{"examples/**"}}
	mods, err := Discover(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(mods))
	for i, m := range mods {
		got[i] = m.Path
	}
	want := []string{"example.com/svc-a", "example.com/svc-b", "example.com/tools/gen"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Discover got %v, want %v", got, want)
	}
}

func TestLintDetectsMismatch(t *testing.T) {
	mods := []Module{
		{Path: "svc-a", GoVersion: "1.25.0", Requires: map[string]string{"github.com/pkg/errors": "v0.9.1"}},
		{Path: "svc-b", GoVersion: "1.24.0", Requires: map[string]string{"github.com/pkg/errors": "v0.8.0"}},
		{Path: "svc-c", GoVersion: "1.25.0", Requires: map[string]string{"svc-a": "v0.0.0"}}, // intra-workspace, ignored
	}
	ms := Lint(mods)

	byDep := map[string]Mismatch{}
	for _, m := range ms {
		byDep[m.Dep] = m
	}
	pe, ok := byDep["github.com/pkg/errors"]
	if !ok {
		t.Fatalf("expected pkg/errors mismatch, got %+v", ms)
	}
	if got := pe.SortedVersions(); got[0] != "v0.9.1" {
		t.Fatalf("highest should sort first, got %v", got)
	}
	if _, ok := byDep[GoDirective]; !ok {
		t.Fatalf("expected go directive mismatch")
	}
	if _, ok := byDep["svc-a"]; ok {
		t.Fatalf("intra-workspace dep should be ignored")
	}
}

func TestFixHighest(t *testing.T) {
	root := t.TempDir()
	da := writeMod(t, root, "svc-a", "module example.com/svc-a\n\ngo 1.25.0\n\nrequire github.com/pkg/errors v0.9.1\n")
	db := writeMod(t, root, "svc-b", "module example.com/svc-b\n\ngo 1.25.0\n\nrequire github.com/pkg/errors v0.8.0\n")
	_ = da

	mods, err := Discover(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	ms := Lint(mods)
	changed := Fix(mods, ms, Highest, nil)
	for _, m := range changed {
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
	}

	data, _ := os.ReadFile(filepath.Join(db, "go.mod"))
	if !strings.Contains(string(data), "v0.9.1") {
		t.Fatalf("svc-b should be bumped to v0.9.1, got:\n%s", data)
	}
	// Re-discover: no mismatches remain.
	mods2, _ := Discover(root, Config{})
	if got := Lint(mods2); len(got) != 0 {
		t.Fatalf("expected no mismatches after fix, got %+v", got)
	}
}

func TestFixPinOverridesStrategy(t *testing.T) {
	root := t.TempDir()
	writeMod(t, root, "svc-a", "module example.com/svc-a\n\ngo 1.25.0\n\nrequire github.com/pkg/errors v0.9.1\n")
	db := writeMod(t, root, "svc-b", "module example.com/svc-b\n\ngo 1.25.0\n\nrequire github.com/pkg/errors v0.8.0\n")

	mods, _ := Discover(root, Config{})
	ms := Lint(mods)
	pins := map[string]string{"github.com/pkg/errors": "v0.8.0"}
	changed := Fix(mods, ms, Highest, pins)
	for _, m := range changed {
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(db, "go.mod"))
	_ = data
	// svc-a should now be pinned down to v0.8.0.
	da, _ := os.ReadFile(filepath.Join(root, "svc-a", "go.mod"))
	if !strings.Contains(string(da), "v0.8.0") {
		t.Fatalf("pin should force svc-a to v0.8.0, got:\n%s", da)
	}
}

func TestHoistReplaces(t *testing.T) {
	root := t.TempDir()
	// svc-a has a local replace (=> ../shared) and a version replace.
	writeMod(t, root, "svc-a",
		"module example.com/svc-a\n\ngo 1.25.0\n\n"+
			"require example.com/shared v1.0.0\n\n"+
			"replace example.com/shared => ../shared\n"+
			"replace github.com/old/lib => github.com/new/lib v1.2.3\n")
	writeMod(t, root, "shared", "module example.com/shared\n\ngo 1.25.0\n")

	mods, err := Discover(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	wf, err := NewWorkFile(mods)
	if err != nil {
		t.Fatal(err)
	}
	SetUseSet(wf, root, mods)
	mutated, warnings := HoistReplaces(root, wf, mods)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(mutated) != 1 {
		t.Fatalf("expected svc-a mutated, got %d", len(mutated))
	}
	for _, m := range mutated {
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteWorkFile(root, wf); err != nil {
		t.Fatal(err)
	}

	work, _ := os.ReadFile(filepath.Join(root, "go.work"))
	ws := string(work)
	if !strings.Contains(ws, "example.com/shared => ./svc-a/../shared") &&
		!strings.Contains(ws, "example.com/shared => ./shared") {
		t.Fatalf("go.work should contain rebased local replace, got:\n%s", ws)
	}
	if !strings.Contains(ws, "github.com/old/lib => github.com/new/lib v1.2.3") {
		t.Fatalf("go.work should contain hoisted version replace, got:\n%s", ws)
	}
	// Replaces gone from svc-a/go.mod.
	amod, _ := os.ReadFile(filepath.Join(root, "svc-a", "go.mod"))
	if strings.Contains(string(amod), "replace") {
		t.Fatalf("svc-a go.mod should have no replace left, got:\n%s", amod)
	}
}

func TestSetUseSetPreservesReplace(t *testing.T) {
	root := t.TempDir()
	writeMod(t, root, "svc-a", "module example.com/svc-a\n\ngo 1.25.0\n")
	// Existing go.work with a replace block.
	os.WriteFile(filepath.Join(root, "go.work"),
		[]byte("go 1.25.0\n\nuse ./old\n\nreplace github.com/x/y => ../y\n"), 0o644)

	wf, err := ReadWorkFile(root)
	if err != nil {
		t.Fatal(err)
	}
	mods, _ := Discover(root, Config{})
	added, removed := SetUseSet(wf, root, mods)
	if len(added) != 1 || added[0] != "./svc-a" {
		t.Fatalf("added=%v", added)
	}
	if len(removed) != 1 || removed[0] != "./old" {
		t.Fatalf("removed=%v", removed)
	}
	out := string(FormatWorkFile(wf))
	if !strings.Contains(out, "replace github.com/x/y") {
		t.Fatalf("replace block should be preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "use ./svc-a") {
		t.Fatalf("use should be updated, got:\n%s", out)
	}
}
