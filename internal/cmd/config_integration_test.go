package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// TestConfigCommandIntegration writes a real two-module workspace to a temp dir
// and runs config-command steps through the executor with live sh/go
// subprocesses, asserting where each step type runs by the files it leaves
// behind (`pwd`/`go:generate` redirect a marker into their working directory).
func TestConfigCommandIntegration(t *testing.T) {
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
	write("go.work", "go 1.25\n\nuse (\n\t./api\n\t./web\n)\n")
	write("api/go.mod", "module example.com/api\n\ngo 1.25\n")
	// a //go:generate directive that drops a marker in its package directory
	write("api/x.go", "package api\n\n//go:generate sh -c \"pwd > gen.txt\"\n")
	write("web/go.mod", "module example.com/web\n\ngo 1.25\n")
	write("web/x.go", "package web\n")

	cfg, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	mods, err := workspace.Discover(root, cfg)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	newP := func() *printer {
		c := &cobra.Command{}
		var buf bytes.Buffer
		c.SetOut(&buf)
		c.SetErr(&buf)
		return newPrinter(c)
	}
	run := func(cc workspace.ConfigCommand) error {
		return execConfigCommand(newP(), root, workspace.Config{}, mods, nil, cc, nil)
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}

	// 1. Bare shell step → runs in the workspace root.
	if err := run(workspace.ConfigCommand{Steps: []string{"pwd > root.txt"}}); err != nil {
		t.Fatalf("bare shell step: %v", err)
	}
	if !exists("root.txt") || exists("api/root.txt") {
		t.Error("bare shell step should run in the root, not a module")
	}

	// 2. Shell step with dir="api" → runs in that module's directory.
	if err := run(workspace.ConfigCommand{Steps: []string{"pwd > scoped.txt"}, Dir: "api"}); err != nil {
		t.Fatalf("dir shell step: %v", err)
	}
	if !exists("api/scoped.txt") || exists("scoped.txt") {
		t.Error("shell step with dir=api should run in the api module")
	}

	// 3. module:verb step → go generate runs in the module (directive cwd = api).
	if err := run(workspace.ConfigCommand{Steps: []string{"api:generate"}}); err != nil {
		t.Fatalf("module:verb step: %v", err)
	}
	if !exists("api/gen.txt") {
		t.Error("api:generate should run go generate in the api module")
	}

	// 4. The fix: a misplaced bare go command fails with a unit:verb hint.
	err = run(workspace.ConfigCommand{Steps: []string{"go build ./..."}})
	if err == nil {
		t.Fatal("bare `go build ./...` from the root should fail")
	}
	var ce *cmdError
	if !errors.As(err, &ce) || !strings.Contains(ce.hint, "<unit>:<verb>") {
		t.Errorf("misplaced go step should carry the unit:verb hint, got: %v", err)
	}
}

// TestConfigModuleResolution proves module:verb resolution against a real
// workspace with two modules sharing the short name "api": the exact module
// path targets that module alone, a unique short name resolves, and the shared
// short name is ambiguous (and runs nothing). Each module marks its own package
// directory via a //go:generate directive.
func TestConfigModuleResolution(t *testing.T) {
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
	gen := "package %s\n\n//go:generate sh -c \"pwd > gen.txt\"\n"
	write("go.work", "go 1.25\n\nuse (\n\t./api\n\t./svc/api\n\t./web\n)\n")
	write("api/go.mod", "module example.com/api\n\ngo 1.25\n")
	write("api/x.go", fmt.Sprintf(gen, "api"))
	write("svc/api/go.mod", "module example.com/svc/api\n\ngo 1.25\n")
	write("svc/api/x.go", fmt.Sprintf(gen, "api"))
	write("web/go.mod", "module example.com/web\n\ngo 1.25\n")
	write("web/x.go", fmt.Sprintf(gen, "web"))

	cfg, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	mods, err := workspace.Discover(root, cfg)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(mods) != 3 {
		t.Fatalf("want 3 modules discovered, got %d", len(mods))
	}

	run := func(step string) error {
		c := &cobra.Command{}
		var buf bytes.Buffer
		c.SetOut(&buf)
		c.SetErr(&buf)
		return execConfigCommand(newPrinter(c), root, workspace.Config{}, mods, nil, workspace.ConfigCommand{Steps: []string{step}}, nil)
	}
	exists := func(rel string) bool { _, err := os.Stat(filepath.Join(root, rel)); return err == nil }
	clean := func() {
		os.Remove(filepath.Join(root, "api/gen.txt"))
		os.Remove(filepath.Join(root, "svc/api/gen.txt"))
		os.Remove(filepath.Join(root, "web/gen.txt"))
	}

	// Exact module path → that module alone.
	if err := run("example.com/api:generate"); err != nil {
		t.Fatalf("exact path: %v", err)
	}
	if !exists("api/gen.txt") || exists("svc/api/gen.txt") {
		t.Error("example.com/api:generate should target ./api alone")
	}
	clean()

	// The nested exact path targets the other module.
	if err := run("example.com/svc/api:generate"); err != nil {
		t.Fatalf("exact nested path: %v", err)
	}
	if !exists("svc/api/gen.txt") || exists("api/gen.txt") {
		t.Error("example.com/svc/api:generate should target ./svc/api alone")
	}
	clean()

	// A unique short name resolves.
	if err := run("web:generate"); err != nil {
		t.Fatalf("unique short name: %v", err)
	}
	if !exists("web/gen.txt") {
		t.Error("web:generate should resolve the unique short name")
	}
	clean()

	// A short name shared by two modules is ambiguous, and runs nothing.
	err = run("api:generate")
	if err == nil {
		t.Fatal("bare 'api' should be ambiguous (api + svc/api)")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("want an ambiguous-module error, got: %v", err)
	}
	if exists("api/gen.txt") || exists("svc/api/gen.txt") {
		t.Error("an ambiguous step must not run anything")
	}
}
