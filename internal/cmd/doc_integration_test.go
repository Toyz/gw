package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// TestDocIntegration drives gw doc's two dispatch paths against a real temp
// workspace: a Go module docs via `go doc` (short-name → import path + symbol),
// and a project docs via its toolchain's `doc` task (run in the project dir).
func TestDocIntegration(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.work", "go 1.25\n\nuse (\n\t./api\n)\n")
	write("api/go.mod", "module example.com/api\n\ngo 1.25\n")
	write("api/api.go", "package api\n\n// Hello greets from the api module.\nfunc Hello() string { return \"hi\" }\n")
	// a project whose "doc" task drops a marker in its own directory
	write("gw.toml", "[projects.sat]\npath = \"sat\"\ntoolchain = \"demo\"\n\n[toolchains.demo]\ndoc = \"pwd > docran.txt\"\n")
	if err := os.MkdirAll(filepath.Join(root, "sat"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	mods, err := workspace.Discover(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	units, _ := workspace.Units(root, mods, cfg.Projects)

	newP := func() (*printer, *bytes.Buffer) {
		c := &cobra.Command{}
		var buf bytes.Buffer
		c.SetOut(&buf)
		c.SetErr(&buf)
		return newPrinter(c), &buf
	}

	// Go module: short name resolves, `go doc <path> Hello` shows the comment.
	u, err := resolveUnit(units, "api")
	if err != nil || !u.IsModule {
		t.Fatalf("resolve api: %v (%+v)", err, u)
	}
	p, buf := newP()
	if err := runGoDoc(p, u.Dir, nil, u.Name, []string{"Hello"}); err != nil {
		t.Fatalf("runGoDoc: %v", err)
	}
	if !strings.Contains(buf.String(), "greets from the api module") {
		t.Errorf("go doc output missing the symbol comment:\n%s", buf.String())
	}

	// Project: its toolchain's doc task runs in the project directory.
	sat, err := resolveUnit(units, "sat")
	if err != nil || sat.IsModule {
		t.Fatalf("resolve sat: %v (%+v)", err, sat)
	}
	pp, _ := newP()
	if err := runProjectDoc(pp, cfg, sat); err != nil {
		t.Fatalf("runProjectDoc: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "sat", "docran.txt")); err != nil {
		t.Error("project doc task should run in the project dir (docran.txt missing)")
	}
}
