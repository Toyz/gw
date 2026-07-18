//go:build integration

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireTool skips the test if a real toolchain isn't installed, so this only
// runs where cargo/uv are present (the integration pipeline).
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not on PATH; run under the integration pipeline", name)
	}
}

// stripNestedGit removes any nested .git directory under root (cargo/uv init
// create their own), so the single workspace repo is authoritative.
func stripNestedGit(t *testing.T, root string) {
	t.Helper()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == ".git" && path != filepath.Join(root, ".git") {
			_ = os.RemoveAll(path)
		}
		return nil
	})
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	c := exec.Command(name, args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// TestRealPolyglot drives the built gw against a REAL polyglot workspace — a Go
// module, an actual Cargo crate, and an actual uv (Python) project — with the
// real cargo/uv toolchains installed. It proves `gw test`/`gw affected` dispatch
// to and succeed with genuine toolchains, not shims. Build-tagged `integration`
// so the default `go test ./...` (which must not need rust/uv) never runs it.
func TestRealPolyglot(t *testing.T) {
	for _, tool := range []string{"go", "git", "cargo", "uv"} {
		requireTool(t, tool)
	}
	gw := buildGW(t)
	root := t.TempDir()

	// A real Go module with a passing test.
	writeFileT(t, filepath.Join(root, "svc/api/go.mod"), "module example.com/api\n\ngo 1.25.0\n")
	writeFileT(t, filepath.Join(root, "svc/api/api.go"), "package api\n\nfunc Add(a, b int) int { return a + b }\n")
	writeFileT(t, filepath.Join(root, "svc/api/api_test.go"), "package api\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")

	// A real Cargo crate (cargo init --lib scaffolds a passing test).
	if err := os.MkdirAll(filepath.Join(root, "sat"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, filepath.Join(root, "sat"), "cargo", "init", "--vcs", "none", "--lib", "--name", "sat")

	// A real uv (Python) project with pytest and a passing test.
	ingest := filepath.Join(root, "py", "ingest")
	if err := os.MkdirAll(ingest, 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, ingest, "uv", "init")
	mustRun(t, ingest, "uv", "add", "--dev", "pytest")
	writeFileT(t, filepath.Join(ingest, "tests", "test_ok.py"), "def test_ok():\n    assert 1 + 1 == 2\n")

	writeFileT(t, filepath.Join(root, "gw.toml"), `
[projects.sat]
path = "sat"
toolchain = "rust"

[projects.ingest]
path = "py/ingest"
toolchain = "uv"

[toolchains.uv]
test = "uv run pytest -q"
`)

	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		c := exec.Command(gw, append([]string{"-C", root}, args...)...)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	// cargo/uv scaffold their own nested .git — drop them so the one workspace
	// repo owns everything.
	stripNestedGit(t, root)

	if out, err := run(t, "init"); err != nil {
		t.Fatalf("gw init: %v\n%s", err, out)
	}
	gitInitT(t, root)

	// gw test must pass across all three REAL toolchains.
	out, err := run(t, "test")
	if err != nil {
		t.Fatalf("gw test (real toolchains) failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0 failed") {
		t.Errorf("gw test should report 0 failed\n%s", out)
	}

	// Editing only the Rust crate → affected is exactly the rust project.
	writeFileT(t, filepath.Join(root, "sat", "src", "lib.rs"), "pub fn v() -> i32 { 2 }\n")
	out, err = run(t, "affected", "--since", "HEAD", "--projects")
	if err != nil {
		t.Fatalf("gw affected: %v\n%s", err, out)
	}
	if got := strings.Fields(out); len(got) != 1 || got[0] != "sat" {
		t.Errorf("affected --projects = %q, want [sat]", out)
	}
}
