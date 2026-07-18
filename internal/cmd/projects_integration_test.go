package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildGW compiles the gw binary once for the integration test.
func buildGW(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleRoot := filepath.Join(wd, "..", "..") // internal/cmd -> repo root
	bin := filepath.Join(t.TempDir(), "gw")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gw: %v\n%s", err, out)
	}
	return bin
}

// writeShim writes a fake toolchain executable that logs "<tool> <args> ::
// cwd=<pwd>" to $GW_TEST_LOG and exits 0 (or the given code).
func writeShim(t *testing.T, dir, tool string, exitCode int) {
	t.Helper()
	script := "#!/bin/sh\n" +
		`echo "` + tool + ` $* :: cwd=$(pwd)" >> "$GW_TEST_LOG"` + "\n" +
		"exit " + itoaTest(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(dir, tool), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	return "1"
}

func writeFileT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInitT(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=a@b.c", "-c", "user.name=x", "commit", "-qm", "init"},
	} {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestProjectsIntegration is the hermetic end-to-end test for the polyglot
// [projects] task runner: fake cargo/uv/blub shims on PATH log their invocation
// + cwd, so we assert dispatch, hook/step order, affected selectivity, command
// args, toolchain overrides and errors without any real Rust/Python toolchain.
func TestProjectsIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	gw := buildGW(t)
	root := t.TempDir()

	shims := filepath.Join(root, ".shims")
	if err := os.MkdirAll(shims, 0o755); err != nil {
		t.Fatal(err)
	}
	writeShim(t, shims, "cargo", 0)
	writeShim(t, shims, "uv", 0)
	writeShim(t, shims, "blub", 0)

	// Fixture: a Go module, a first-party rust project, a user-toolchain uv
	// project, plus a command with args and a hook.
	writeFileT(t, filepath.Join(root, "svc/api/go.mod"), "module example.com/api\n\ngo 1.25.0\n")
	writeFileT(t, filepath.Join(root, "svc/api/api.go"), "package api\n\nfunc F() int { return 1 }\n")
	writeFileT(t, filepath.Join(root, "sat/src/main.rs"), "fn main() {}\n")
	writeFileT(t, filepath.Join(root, "py/ingest/m.py"), "print('hi')\n")
	writeFileT(t, filepath.Join(root, "gw.toml"), `
[projects.sat]
path = "sat"
toolchain = "rust"

[projects.ingest]
path = "py/ingest"
toolchain = "uv"

[toolchains.uv]
build = "uv build"
test  = "uv run pytest"

[commands.deploy]
desc = "deploy one unit"
args = ["service"]
steps = ["${service}:build", "blub ship $service $1"]

[hooks.pre-test]
steps = ["blub pretest"]
`)

	logf := filepath.Join(root, "log")
	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		_ = os.WriteFile(logf, nil, 0o644)
		c := exec.Command(gw, append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(),
			"PATH="+shims+string(os.PathListSeparator)+os.Getenv("PATH"),
			"GW_TEST_LOG="+logf,
		)
		out, err := c.CombinedOutput()
		return string(out), err
	}
	readLog := func(t *testing.T) string {
		t.Helper()
		b, _ := os.ReadFile(logf)
		return string(b)
	}

	// Bootstrap the workspace (go.work), no network needed.
	if out, err := run(t, "init"); err != nil {
		t.Fatalf("gw init: %v\n%s", err, out)
	}
	gitInitT(t, root)

	rel := func(p string) string { return filepath.Join(root, p) }

	// 1+2. Dispatch + cwd + hook order: `gw test` runs go (real) in svc/api,
	// cargo test in sat, uv run pytest in py/ingest, with the pre-test hook first.
	out, err := run(t, "test")
	if err != nil {
		t.Fatalf("gw test: %v\n%s", err, out)
	}
	log := readLog(t)
	for _, want := range []string{
		"blub pretest",
		"cargo test :: cwd=" + rel("sat"),
		"uv run pytest :: cwd=" + rel("py/ingest"),
	} {
		if !strings.Contains(log, want) {
			t.Errorf("gw test log missing %q\nlog:\n%s", want, log)
		}
	}
	// hook fires before the toolchain tasks
	if i, j := strings.Index(log, "blub pretest"), strings.Index(log, "cargo test"); i < 0 || j < 0 || i > j {
		t.Errorf("pre-test hook should run before tasks\nlog:\n%s", log)
	}

	// 3. Affected: editing only the rust project reports just it.
	writeFileT(t, rel("sat/src/main.rs"), "fn main() { /* v2 */ }\n")
	out, err = run(t, "affected", "--since", "HEAD", "--projects")
	if err != nil {
		t.Fatalf("gw affected: %v\n%s", err, out)
	}
	if got := strings.Fields(out); len(got) != 1 || got[0] != "sat" {
		t.Errorf("affected --projects = %q, want [sat]", out)
	}

	// 4. Command args: gw deploy sat -> ${service}:build (cargo build in sat) and
	// a shell step with $service (env) + $1 (positional).
	out, err = run(t, "deploy", "sat")
	if err != nil {
		t.Fatalf("gw deploy: %v\n%s", err, out)
	}
	log = readLog(t)
	if !strings.Contains(log, "cargo build :: cwd="+rel("sat")) {
		t.Errorf("deploy: ${service}:build should cargo build in sat\nlog:\n%s", log)
	}
	if !strings.Contains(log, "blub ship sat sat ::") {
		t.Errorf("deploy: shell step should bind $service and $1 to sat\nlog:\n%s", log)
	}

	// 5. Wrong arg count -> usage error.
	if _, err := run(t, "deploy"); err == nil {
		t.Error("gw deploy with no args should error (ExactArgs)")
	}
}
