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

	flags map[string]any // parsed typed flags (see Command flags / c.String etc.)
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

// Builtin invokes gw's own builtin command of the given name, streaming stdio.
// It is the call-through primitive for Override handlers: decorate a builtin by
// handling your added flags, then fall through to the original behavior with
// Builtin. Extensions are disabled in the child process, so an Override of "run"
// that calls c.Builtin("run", ...) reaches the real builtin without re-entering
// itself. Returns an error if gw did not advertise its own path (GW_BIN).
func (c *Context) Builtin(name string, args ...string) error {
	bin := os.Getenv("GW_BIN")
	if bin == "" {
		return fmt.Errorf("gwext: GW_BIN not set; cannot call builtin %q", name)
	}
	argv := []string{}
	if c.Root != "" {
		argv = append(argv, "-C", c.Root)
	}
	argv = append(argv, name)
	argv = append(argv, args...)
	cmd := exec.Command(bin, argv...)
	cmd.Dir = c.Root
	// GW_SKIP_EXT tells the child gw to run its pure builtin tree: no extension
	// commands, no overrides, no hooks — which prevents this override recursing.
	cmd.Env = append(os.Environ(), "GW_SKIP_EXT=1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CommandInfo describes a registered custom command. Override is set when the
// command intentionally replaces a builtin of the same name; Passthrough when it
// forwards undeclared flags to c.Args (see Cmd.Passthrough).
type CommandInfo struct {
	Name        string `json:"name"`
	Short       string `json:"short"`
	Override    bool   `json:"override,omitempty"`
	Passthrough bool   `json:"passthrough,omitempty"`
	Flags       []Flag `json:"flags,omitempty"`
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

// ProvideResult is the full output of the `provide` verb: the global BuildInfo
// (embedded, so its env/vars/tags stay at the top level for backward compat)
// plus per-module overrides keyed by module path, from ProvideEach.
type ProvideResult struct {
	BuildInfo
	Each map[string]BuildInfo `json:"each,omitempty"`
}

// merge folds src into dst (env/vars maps, tags concatenated).
func (dst *BuildInfo) merge(src BuildInfo) {
	for k, v := range src.Env {
		if dst.Env == nil {
			dst.Env = map[string]string{}
		}
		dst.Env[k] = v
	}
	for k, v := range src.Vars {
		if dst.Vars == nil {
			dst.Vars = map[string]string{}
		}
		dst.Vars[k] = v
	}
	dst.Tags = append(dst.Tags, src.Tags...)
}

// empty reports whether a BuildInfo contributes nothing.
func (b BuildInfo) empty() bool {
	return len(b.Env) == 0 && len(b.Vars) == 0 && len(b.Tags) == 0
}

type commandReg struct {
	info        CommandInfo
	run         func(*Context) error
	flags       []Flag
	passthrough bool
}

var (
	commands      []*commandReg
	hooks         = map[string][]func(*Context) error{}
	providers     []func(*Context) (BuildInfo, error)
	eachProviders []func(*Context, Module) (BuildInfo, error)
	hidden        []string
)

// Command registers a custom subcommand, invocable as `gw <name>`. A name that
// collides with a builtin is ignored by gw; use Override to replace a builtin.
// Optional typed flags are declared with Str/Bool/Int; gw parses them from the
// user's args and the handler reads them via c.String/c.Bool/c.Int.
func Command(name, short string, run func(*Context) error, flags ...Flag) Cmd {
	c := &commandReg{info: CommandInfo{Name: name, Short: short, Flags: flags}, run: run, flags: flags}
	commands = append(commands, c)
	return Cmd{c}
}

// Override registers a subcommand that intentionally replaces the builtin of the
// same name (e.g. to wrap `gw test` with setup/teardown). Unlike Command, gw
// removes the shadowed builtin instead of skipping the extension command.
func Override(name, short string, run func(*Context) error, flags ...Flag) Cmd {
	c := &commandReg{info: CommandInfo{Name: name, Short: short, Override: true, Flags: flags}, run: run, flags: flags}
	commands = append(commands, c)
	return Cmd{c}
}

// Cmd is the handle returned by Command and Override for optional configuration.
type Cmd struct{ reg *commandReg }

// Passthrough makes the command forward any flags it does not declare — plus
// positional args — into c.Args instead of erroring on them. Use it when
// overriding a builtin that hands flags to the go tool (build/test/run/vet), so
// the override can add its own flags and still forward the rest to c.Builtin:
//
//	gwext.Override("build", "generate, then build",
//		func(c *gwext.Context) error {
//			for _, m := range c.Modules {
//				if err := c.Mod(m.Path).Generate(); err != nil {
//					return err
//				}
//			}
//			return c.Builtin("build", c.Args...)
//		},
//		gwext.Bool("gen", "run go generate first")).Passthrough()
//
// `gw build --gen -p ./...` then sets gen and forwards `-p ./...` untouched.
func (c Cmd) Passthrough() Cmd {
	c.reg.passthrough = true
	c.reg.info.Passthrough = true
	return c
}

// Hide removes the named builtin commands from gw's command tree for this
// workspace. Hiding `ext` still leaves extension auto-build intact.
func Hide(names ...string) {
	hidden = append(hidden, names...)
}

// Hook registers a function to run at a lifecycle event (e.g. "pre-sync"),
// appended in registration order.
//
// Deprecated: use the typed Before/After helpers — they build the event name
// for you, so the "pre-"/"post-" phase and the "-" separator can't be mistyped.
// Hook still works for arbitrary event strings.
func Hook(event string, run func(*Context) error) { registerHook(event, run) }

// Before registers a hook that runs immediately before the named command —
// pre-<command>. command is any gw command: a builtin (use the constants below,
// e.g. gwext.Build) or one of your own, gwext.Before("deploy", fn).
func Before(command string, run func(*Context) error) { registerHook("pre-"+command, run) }

// After registers a hook that runs after the named command completes
// successfully — post-<command>.
func After(command string, run func(*Context) error) { registerHook("post-"+command, run) }

func registerHook(event string, run func(*Context) error) {
	hooks[event] = append(hooks[event], run)
}

// Builtin command names, for typo-safe use with Before/After. Hooks work for any
// command, so custom verbs just take a string: gwext.Before("deploy", fn).
const (
	Init     = "init"
	Sync     = "sync"
	Lint     = "lint"
	Doctor   = "doctor"
	Verify   = "verify"
	Build    = "build"
	Test     = "test"
	Vet      = "vet"
	Generate = "generate"
	Tidy     = "tidy"
	Run      = "run"
	List     = "list"
	Graph    = "graph"
	Affected = "affected"
	Add      = "add"
	Remove   = "remove"
)

// Provide registers a build provider: a function that computes a BuildInfo
// (environment variables, -ldflags -X vars, build tags) at run time, which gw
// folds into its commands. It is gw's analogue of a Cargo build script emitting
// cargo::rustc-env / rustc-cfg. Multiple providers merge in registration order;
// providers may print freely (that output is routed to stderr, not the result).
func Provide(fn func(*Context) (BuildInfo, error)) {
	providers = append(providers, fn)
}

// ProvideEach registers a per-module build provider: gw calls it once for each
// workspace module and applies the returned BuildInfo to that module alone.
// Return a zero BuildInfo to contribute nothing for a given module. Use it to
// scope env/tags/vars to specific modules — e.g. a "server" build tag only on
// your service modules. Global Provide output still applies to every module;
// per-module values layer on top of it.
func ProvideEach(fn func(*Context, Module) (BuildInfo, error)) {
	eachProviders = append(eachProviders, fn)
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
	m := Manifest{Providers: len(providers) + len(eachProviders)}
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

// emitProvide runs the global providers (merged into ProvideResult.BuildInfo)
// and the per-module providers (merged into ProvideResult.Each[path]), then
// writes the result as JSON. Provider stdout is redirected to stderr while they
// run so a stray print cannot corrupt the JSON result gw reads.
func emitProvide() {
	ctx := contextFromEnv()
	real := os.Stdout
	os.Stdout = os.Stderr
	failBack := func(err error) {
		if err != nil {
			os.Stdout = real
			fail("gwext: build provider: " + err.Error())
		}
	}

	var res ProvideResult
	for _, p := range providers {
		bi, err := p(ctx)
		failBack(err)
		res.BuildInfo.merge(bi)
	}
	if len(eachProviders) > 0 {
		res.Each = map[string]BuildInfo{}
		for _, m := range ctx.Modules {
			var acc BuildInfo
			for _, pe := range eachProviders {
				bi, err := pe(ctx, m)
				failBack(err)
				acc.merge(bi)
			}
			if !acc.empty() {
				res.Each[m.Path] = acc
			}
		}
	}

	os.Stdout = real
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fail("gwext: " + err.Error())
	}
}

func runCommand(name string, userArgs []string) {
	for _, c := range commands {
		if c.info.Name == name {
			ctx := contextFromEnv()
			ctx.Args = userArgs
			if len(c.flags) > 0 {
				if c.passthrough {
					ctx.flags, ctx.Args = parseFlagsPassthrough(name, c.flags, userArgs)
				} else {
					ctx.flags, ctx.Args = parseFlags(name, c.flags, userArgs)
				}
			}
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
	if n := len(providers) + len(eachProviders); n > 0 {
		fmt.Printf("  provider %d build provider(s)\n", n)
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
