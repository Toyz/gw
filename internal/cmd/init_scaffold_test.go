package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitConfigScaffold exercises `gw init --config` (go.work + gw.toml, no
// network) and its idempotent re-run. --ext is not covered here because it runs
// `go get` (network); the real integration pipeline covers ext scaffolding.
func TestInitConfigScaffold(t *testing.T) {
	gw := buildGW(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFileT(t, filepath.Join(root, "m/go.mod"), "module example.com/m\n\ngo 1.25.0\n")
	writeFileT(t, filepath.Join(root, "m/m.go"), "package m\n")

	run := func() (string, error) {
		c := exec.Command(gw, "-C", root, "init", "--config")
		out, err := c.CombinedOutput()
		return string(out), err
	}

	if out, err := run(); err != nil {
		t.Fatalf("init --config: %v\n%s", err, out)
	}
	for _, f := range []string{"go.work", "gw.toml"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
	// Re-run: both already exist -> skipped, no error.
	out, err := run()
	if err != nil {
		t.Fatalf("re-run init --config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already exists; skipped") {
		t.Errorf("re-run should skip existing files, got:\n%s", out)
	}
}
