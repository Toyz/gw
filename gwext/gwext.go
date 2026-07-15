// Package gwext is the SDK for gw extensions. Author a .gw/build.go that imports
// this package, registers custom commands and lifecycle hooks, and calls Main:
//
//	package main
//
//	import "github.com/toyz/gw/gwext"
//
//	func main() {
//		gwext.Command("hello", "print a greeting", func(c *gwext.Context) error {
//			fmt.Printf("hello from %d modules\n", len(c.Modules))
//			return nil
//		})
//		gwext.Hook("post-sync", func(c *gwext.Context) error {
//			fmt.Println("go.work synced")
//			return nil
//		})
//		gwext.Main()
//	}
//
// gw compiles the file and talks to the resulting binary over a small JSON
// protocol; extension authors never invoke that protocol directly.
package gwext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

// Module mirrors a workspace module as gw sees it. gw streams the module set to
// extensions over stdin; extension code reads it through Context.Modules.
type Module struct {
	Path      string            `json:"path"`
	Dir       string            `json:"dir"`
	GoVersion string            `json:"go,omitempty"`
	Toolchain string            `json:"toolchain,omitempty"`
	Requires  map[string]string `json:"requires,omitempty"`
}

// Context is handed to every command and hook. It carries the workspace root,
// the discovered modules, the user-supplied args (commands) and the triggering
// event name (hooks).
type Context struct {
	Root    string
	Modules []Module
	Args    []string
	Event   string
}

// Module returns the workspace module with the given path (and ok=false if none).
func (c *Context) Module(path string) (Module, bool) {
	for _, m := range c.Modules {
		if m.Path == path {
			return m, true
		}
	}
	return Module{}, false
}

// Dir returns the on-disk directory of a workspace module by path. It panics if
// the module is unknown; use Module for a checked lookup.
func (c *Context) Dir(modulePath string) string {
	m, ok := c.Module(modulePath)
	if !ok {
		panic("gwext: unknown module " + modulePath)
	}
	return m.Dir
}

// Run executes name+args in dir, streaming stdio, and returns any error. An empty
// dir runs in the workspace root. This is the building block for orchestration:
// call it in whatever order (serially, or from goroutines for parallel steps).
func (c *Context) Run(dir, name string, args ...string) error {
	if dir == "" {
		dir = c.Root
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunIn executes name+args in the directory of the named workspace module.
func (c *Context) RunIn(modulePath, name string, args ...string) error {
	m, ok := c.Module(modulePath)
	if !ok {
		return fmt.Errorf("gwext: unknown module %q", modulePath)
	}
	return c.Run(m.Dir, name, args...)
}

// Start launches name+args in dir without waiting, returning the running command
// so callers can orchestrate long-lived processes (e.g. servers) and stop them
// later with cmd.Process.Kill / cmd.Wait. An empty dir runs in the workspace root.
func (c *Context) Start(dir, name string, args ...string) (*exec.Cmd, error) {
	if dir == "" {
		dir = c.Root
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, cmd.Start()
}

// Go runs `go <args>` in dir (empty dir = workspace root), streaming stdio.
func (c *Context) Go(dir string, args ...string) error {
	return c.Run(dir, "go", args...)
}

// Mod returns a typed handle for orchestrating a single workspace module by path.
// Calling a method on an unknown module returns an error rather than panicking.
func (c *Context) Mod(path string) *ModuleRef {
	m, ok := c.Module(path)
	return &ModuleRef{ctx: c, mod: m, ok: ok, path: path}
}

// ModuleRef is a fluent handle to one workspace module. Its Go-tool methods
// (Build/Test/Run/Tidy/Vet/Generate) default to `./...` when no packages are
// given, so `c.Mod("x").Build()` just works; Exec runs any binary (polyglot).
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

// CommandInfo describes a registered custom command. Override is set when the
// command intentionally replaces a builtin of the same name.
type CommandInfo struct {
	Name     string `json:"name"`
	Short    string `json:"short"`
	Override bool   `json:"override,omitempty"`
}

// Manifest is what gw reads to learn an extension's commands, hooks, build
// providers, and any builtins it hides.
type Manifest struct {
	Commands  []CommandInfo `json:"commands"`
	Hooks     []string      `json:"hooks"`
	Providers int           `json:"providers"`
	Hidden    []string      `json:"hidden,omitempty"`
}

// BuildInfo is what a Provide function contributes to gw's commands, computed at
// run time (a git SHA, a build stamp, a derived tag). It is gw's analogue of a
// Cargo build script's cargo::rustc-env / rustc-cfg output.
type BuildInfo struct {
	// Env sets process environment variables for every command gw runs
	// (build/test/vet/generate/tidy, run, and extension commands).
	Env map[string]string `json:"env,omitempty"`
	// Vars are stamped into the binary via `-ldflags "-X <key>=<value>"` for the
	// compiling commands (build, test). Keys are `importpath.Name`.
	Vars map[string]string `json:"vars,omitempty"`
	// Tags are build tags added to the compiling commands (build, test, vet).
	Tags []string `json:"tags,omitempty"`
}

type commandReg struct {
	info CommandInfo
	run  func(*Context) error
}

var (
	commands  []commandReg
	hooks     = map[string][]func(*Context) error{}
	providers []func(*Context) (BuildInfo, error)
	hidden    []string
)

// Command registers a custom subcommand, invocable as `gw <name>`. A name that
// collides with a builtin is ignored by gw; use Override to replace a builtin.
func Command(name, short string, run func(*Context) error) {
	commands = append(commands, commandReg{CommandInfo{Name: name, Short: short}, run})
}

// Override registers a subcommand that intentionally replaces the builtin of the
// same name (e.g. to wrap `gw test` with setup/teardown). Unlike Command, gw
// removes the shadowed builtin instead of skipping the extension command.
func Override(name, short string, run func(*Context) error) {
	commands = append(commands, commandReg{CommandInfo{Name: name, Short: short, Override: true}, run})
}

// Hide removes the named builtin commands from gw's command tree for this
// workspace. Hiding `ext` still leaves extension auto-build intact.
func Hide(names ...string) {
	hidden = append(hidden, names...)
}

// Hook registers a function to run at a lifecycle event (e.g. "pre-sync",
// "post-lint"). Multiple hooks per event run in registration order.
func Hook(event string, run func(*Context) error) {
	hooks[event] = append(hooks[event], run)
}

// Provide registers a build provider: a function that computes a BuildInfo
// (environment variables, -ldflags -X vars, build tags) at run time, which gw
// folds into its commands. It is gw's analogue of a Cargo build script emitting
// cargo::rustc-env / rustc-cfg. Multiple providers merge in registration order;
// providers may print freely (that output is routed to stderr, not the result).
func Provide(fn func(*Context) (BuildInfo, error)) {
	providers = append(providers, fn)
}

// The protocol sentinel keeps gw's calls from colliding with user arguments.
const sentinel = "__gwext"

// Main dispatches the gw<->extension protocol. Call it once, last, in main.
// Run directly by a human, it prints the manifest in readable form.
func Main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != sentinel {
		printHuman()
		return
	}
	if len(args) < 2 {
		fail("gwext: missing protocol verb")
	}
	switch args[1] {
	case "manifest":
		emitManifest()
	case "command":
		if len(args) < 3 {
			fail("gwext: command requires a name")
		}
		runCommand(args[2], args[3:])
	case "hook":
		if len(args) < 3 {
			fail("gwext: hook requires an event")
		}
		runHook(args[2])
	case "provide":
		emitProvide()
	default:
		fail("gwext: unknown verb " + args[1])
	}
}

func emitManifest() {
	m := Manifest{Providers: len(providers)}
	for _, c := range commands {
		m.Commands = append(m.Commands, c.info)
	}
	for e := range hooks {
		m.Hooks = append(m.Hooks, e)
	}
	m.Hidden = dedupSorted(hidden)
	sort.Strings(m.Hooks)
	sort.Slice(m.Commands, func(i, j int) bool { return m.Commands[i].Name < m.Commands[j].Name })
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		fail("gwext: " + err.Error())
	}
}

// emitProvide runs every registered build provider, merges their BuildInfo, and
// writes the result as JSON on stdout. Provider stdout is redirected to stderr
// while they run so a stray print cannot corrupt the JSON result gw reads.
func emitProvide() {
	ctx := contextFromEnv()
	real := os.Stdout
	os.Stdout = os.Stderr
	merged := BuildInfo{Env: map[string]string{}, Vars: map[string]string{}}
	for _, p := range providers {
		bi, err := p(ctx)
		if err != nil {
			os.Stdout = real
			fail("gwext: build provider: " + err.Error())
		}
		for k, v := range bi.Env {
			merged.Env[k] = v
		}
		for k, v := range bi.Vars {
			merged.Vars[k] = v
		}
		merged.Tags = append(merged.Tags, bi.Tags...)
	}
	os.Stdout = real
	if err := json.NewEncoder(os.Stdout).Encode(merged); err != nil {
		fail("gwext: " + err.Error())
	}
}

func runCommand(name string, userArgs []string) {
	for _, c := range commands {
		if c.info.Name == name {
			ctx := contextFromEnv()
			ctx.Args = userArgs
			if err := c.run(ctx); err != nil {
				fail("gw " + name + ": " + err.Error())
			}
			return
		}
	}
	fail("gwext: no such command " + name)
}

func runHook(event string) {
	ctx := contextFromEnv()
	ctx.Event = event
	for _, h := range hooks[event] {
		if err := h(ctx); err != nil {
			fail("gw hook " + event + ": " + err.Error())
		}
	}
}

func contextFromEnv() *Context {
	ctx := &Context{Root: os.Getenv("GW_ROOT")}
	// gw streams the module set over stdin (env vars have a size ceiling a large
	// workspace would exceed). An empty payload just yields no modules.
	if data, err := io.ReadAll(os.Stdin); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &ctx.Modules)
	}
	return ctx
}

func printHuman() {
	fmt.Println("gw extension. Registered:")
	for _, c := range commands {
		kind := "command"
		if c.info.Override {
			kind = "override"
		}
		fmt.Printf("  %s  %-16s %s\n", kind, c.info.Name, c.info.Short)
	}
	evs := make([]string, 0, len(hooks))
	for e := range hooks {
		evs = append(evs, e)
	}
	sort.Strings(evs)
	for _, e := range evs {
		fmt.Printf("  hook     %s\n", e)
	}
	if len(providers) > 0 {
		fmt.Printf("  provider %d build provider(s)\n", len(providers))
	}
	for _, h := range dedupSorted(hidden) {
		fmt.Printf("  hides    %s\n", h)
	}
	fmt.Println("\nThis binary is driven by gw; run `gw <command>` instead.")
}

// dedupSorted returns the sorted, de-duplicated, non-empty members of in.
func dedupSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
