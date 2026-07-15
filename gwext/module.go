package gwext

import (
	"fmt"
	"os/exec"
)

// Mod returns a typed handle for orchestrating a single workspace module by path.
// Calling a method on an unknown module returns an error rather than panicking.
func (c *Context) Mod(path string) *ModuleRef {
	m, ok := c.Module(path)
	return &ModuleRef{ctx: c, mod: m, ok: ok, path: path}
}

// ModuleRef is a fluent handle to one workspace module. Its Go-tool methods
// (Build/Test/Run/Tidy/Vet/Generate) default to `./...` when no packages are
// given, so `c.Mod("x").Build()` just works. Exec runs any binary, and
// Tool/Npm/Yarn/... wrap other toolchains (see the toolchain section below).
type ModuleRef struct {
	ctx  *Context
	mod  Module
	ok   bool
	path string
}

// Exists reports whether the referenced module is in the workspace.
func (r *ModuleRef) Exists() bool { return r.ok }

// Dir returns the module's directory ("" if the module is unknown).
func (r *ModuleRef) Dir() string { return r.mod.Dir }

// Path returns the module path.
func (r *ModuleRef) Path() string { return r.path }

func (r *ModuleRef) err() error {
	if !r.ok {
		return fmt.Errorf("gwext: unknown module %q", r.path)
	}
	return nil
}

// Go runs `go <args>` in the module's directory.
func (r *ModuleRef) Go(args ...string) error {
	if err := r.err(); err != nil {
		return err
	}
	return r.ctx.Go(r.mod.Dir, args...)
}

// Exec runs an arbitrary command in the module's directory (for polyglot stacks:
// npm, cargo, make, docker, ...).
func (r *ModuleRef) Exec(name string, args ...string) error {
	if err := r.err(); err != nil {
		return err
	}
	return r.ctx.Run(r.mod.Dir, name, args...)
}

// Start launches an arbitrary long-lived command in the module's directory
// without waiting (e.g. a server), returning the running command.
func (r *ModuleRef) Start(name string, args ...string) (*exec.Cmd, error) {
	if err := r.err(); err != nil {
		return nil, err
	}
	return r.ctx.Start(r.mod.Dir, name, args...)
}

func withDefault(args []string, def ...string) []string {
	if len(args) == 0 {
		return def
	}
	return args
}

// --- Go toolchain ----------------------------------------------------------

// Build runs `go build` (default ./...) in the module.
func (r *ModuleRef) Build(pkgs ...string) error {
	return r.Go(append([]string{"build"}, withDefault(pkgs, "./...")...)...)
}

// Test runs `go test` (default ./...) in the module.
func (r *ModuleRef) Test(args ...string) error {
	return r.Go(append([]string{"test"}, withDefault(args, "./...")...)...)
}

// Vet runs `go vet` (default ./...) in the module.
func (r *ModuleRef) Vet(args ...string) error {
	return r.Go(append([]string{"vet"}, withDefault(args, "./...")...)...)
}

// Run runs `go run` (default .) in the module.
func (r *ModuleRef) Run(args ...string) error {
	return r.Go(append([]string{"run"}, withDefault(args, ".")...)...)
}

// Generate runs `go generate` (default ./...) in the module.
func (r *ModuleRef) Generate(args ...string) error {
	return r.Go(append([]string{"generate"}, withDefault(args, "./...")...)...)
}

// Tidy runs `go mod tidy` in the module.
func (r *ModuleRef) Tidy() error { return r.Go("mod", "tidy") }

// --- Other toolchains ------------------------------------------------------

// Tool binds any executable to the module, to run it in the module's directory.
// It is the generic escape hatch for non-go stacks — npm, yarn, cargo, make,
// docker, deno, ... — with no hard-coded tool list:
//
//	c.Mod("web").Tool("yarn").Run("dev")
//	c.Mod("api").Tool("docker").Run("build", ".")
func (r *ModuleRef) Tool(bin string) *Tool { return &Tool{ref: r, bin: bin} }

// Tool runs one executable in a module's directory.
type Tool struct {
	ref *ModuleRef
	bin string
}

// Run runs `bin <args>` in the module's directory.
func (t *Tool) Run(args ...string) error { return t.ref.Exec(t.bin, args...) }

// Start launches `bin <args>` without waiting (e.g. a dev server), returning the
// running command.
func (t *Tool) Start(args ...string) (*exec.Cmd, error) { return t.ref.Start(t.bin, args...) }
