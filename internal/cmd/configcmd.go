package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// attachConfigCommands registers the workspace's config-declared commands and
// records its hook events, so gw.toml can add custom verbs and lifecycle hooks
// with no compiled .gw/build.go. It runs after attachExtCommands, so a compiled
// extension wins any name/event collision and config fills the rest. Best-effort:
// config problems warn and leave the command tree intact.
func attachConfigCommands(rootCmd *cobra.Command) {
	root, err := earlyResolveRoot()
	if err != nil {
		return
	}
	cfg, err := workspace.LoadConfig(root)
	if err != nil {
		return // a real command that loads the workspace will surface it
	}
	if len(cfg.Commands) == 0 && len(cfg.Hooks) == 0 {
		return
	}
	p := newPrinter(rootCmd)

	taken := map[string]bool{} // builtins + any compiled-extension commands
	for _, c := range rootCmd.Commands() {
		taken[c.Name()] = true
	}
	for name, cc := range cfg.Commands {
		switch {
		case taken[name]:
			p.warnf("config command %q is shadowed by a builtin or extension; skipped", name)
			continue
		case cc.Empty():
			p.warnf("config command %q declares no steps or run; skipped", name)
			continue
		}
		short := cc.Desc
		if short == "" {
			short = "workspace command"
		}
		name, cc := name, cc // capture per iteration
		use := name
		argsRule := cobra.ArbitraryArgs // positional args reachable as $1..$@
		if len(cc.Args) > 0 {
			for _, a := range cc.Args {
				use += " <" + a + ">"
			}
			argsRule = cobra.ExactArgs(len(cc.Args))
		}
		rootCmd.AddCommand(&cobra.Command{
			Use:   use,
			Short: short + " (config)",
			Args:  argsRule,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runConfigCommand(newPrinter(cmd), cc, args)
			},
		})
	}

	// Record hook events into the shared gate; fireHook runs them natively.
	for event, cc := range cfg.Hooks {
		if cc.Empty() {
			p.warnf("config hook %q declares no steps or run; skipped", event)
			continue
		}
		addHookEvent(event)
	}
}

// runConfigCommand loads the workspace and executes a config command end to end,
// binding the invocation's CLI args to its steps.
func runConfigCommand(p *printer, cc workspace.ConfigCommand, args []string) error {
	root, cfg, mods, err := loadWorkspaceEarly()
	if err != nil {
		return err
	}
	env, err := workspace.ResolveConfigEnv(root, cfg)
	if err != nil {
		return err
	}
	return execConfigCommand(p, root, cfg, mods, env, cc, args)
}

// execConfigCommand runs a command/hook body: each step in order. A step is
// either a "<unit>:<verb>" op (run in that unit's directory via its toolchain) or
// a shell command (run in the command's dir, else the root). args are the CLI
// args of an invoked command (nil for hooks): they bind as named ($service) via
// env and positional ($1, $@) to shell steps, and are substituted into unit-step
// refs so a unit can be chosen dynamically. Shared by commands + hooks.
func execConfigCommand(p *printer, root string, cfg workspace.Config, mods []workspace.Module, env []string, cc workspace.ConfigCommand, args []string) error {
	shellDir := root
	if cc.Dir != "" {
		d, err := resolveDir(root, mods, cc.Dir)
		if err != nil {
			return err
		}
		shellDir = d
	}
	units, _ := workspace.Units(root, mods, cfg.Projects)
	named := namedArgs(cc.Args, args)
	for _, step := range cc.Steps {
		// Substitute args only to classify + resolve a unit ref; shell steps run
		// their ORIGINAL text with args passed safely (no interpolation).
		if ref, verb, isUnit := parseStep(substituteArgs(step, named, args)); isUnit {
			if err := runUnitStep(p, cfg, units, env, ref, verb); err != nil {
				return err
			}
			continue
		}
		p.step("%s", step)
		if err := runShell(shellDir, env, step, args, named); err != nil {
			e := failf("`%s` failed: %v", step, err)
			if cc.Dir == "" && looksMisplacedGoCmd(step) {
				// Bare `go build/generate/... ./...` (or `go mod tidy`) run from the
				// workspace root fails — the root has no go.mod. Point at the fix.
				e = e.withHint(`go tools are module-relative — use a "<unit>:<verb>" step (e.g. api:generate), or set dir`)
			}
			return e
		}
	}
	return nil
}

// namedArgs pairs declared arg names with the provided CLI args.
func namedArgs(names, args []string) map[string]string {
	m := make(map[string]string, len(names))
	for i, n := range names {
		if i < len(args) {
			m[n] = args[i]
		}
	}
	return m
}

// substituteArgs replaces ${name}/$name and $1..$N in a step string. Used to
// resolve a dynamic unit ref (e.g. "${service}:build"); shell steps receive args
// via env/positional instead, so this never touches their script text.
func substituteArgs(s string, named map[string]string, args []string) string {
	for k, v := range named {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
		s = strings.ReplaceAll(s, "$"+k, v)
	}
	for i := len(args); i >= 1; i-- { // high-to-low so $10 isn't clipped by $1
		s = strings.ReplaceAll(s, "$"+strconv.Itoa(i), args[i-1])
	}
	return s
}

// looksMisplacedGoCmd reports whether a shell step is a module-relative go
// command (a `./...` build/test/vet/generate, or `go mod tidy`) — the kind that
// fails from the workspace root and wants a "<module>:<verb>" step instead.
func looksMisplacedGoCmd(step string) bool {
	f := strings.Fields(step)
	if len(f) < 2 || f[0] != "go" {
		return false
	}
	if f[1] == "mod" {
		return len(f) > 2 && f[2] == "tidy"
	}
	switch f[1] {
	case "build", "test", "vet", "generate":
		for _, a := range f[2:] {
			if a == "./..." {
				return true
			}
		}
	}
	return false
}

// parseStep classifies a step: a whitespace-free "<unit>:<verb>" whose verb is a
// known gw verb is a unit op; anything else is a shell command.
func parseStep(step string) (unitRef, verb string, isUnit bool) {
	if strings.ContainsAny(step, " \t") {
		return "", "", false
	}
	unitRef, verb, ok := strings.Cut(step, ":")
	if !ok || unitRef == "" {
		return "", "", false
	}
	if !workspace.KnownVerbs[verb] {
		return "", "", false
	}
	return unitRef, verb, true
}

// runUnitStep runs a "<unit>:<verb>" op via the unit's toolchain (go/rust argv,
// or a shell command for user toolchains and overrides) in the unit's directory.
func runUnitStep(p *printer, cfg workspace.Config, units []workspace.Unit, env []string, unitRef, verb string) error {
	u, err := resolveUnit(units, unitRef)
	if err != nil {
		return err
	}
	argv, shell, err := workspace.TaskCommand(cfg, u, verb)
	if err != nil {
		return failf("%s: %v", unitRef, err)
	}
	job := workspace.Job{Name: u.Name, Dir: u.Dir, Env: env}
	if shell != "" {
		job.Argv = []string{"sh", "-c", shell}
	} else {
		job.Argv = argv
	}
	results := workspace.RunAcross(context.Background(), []workspace.Job{job}, workspace.ExecOpts{
		Stdout: p.Out(),
		Stderr: p.Err(),
		Header: func(mod string) string {
			return p.s.cyan("→") + " " + p.s.bold(mod) + " " + p.s.dim(verb)
		},
	})
	if workspace.WorstExit(results) != 0 {
		return failf("step %q failed", unitRef+":"+verb)
	}
	return nil
}

// runShell runs a shell script (sh -c) in dir with the workspace env layered on.
// args become the shell's positional params ($1, $@); named binds them as env
// vars ($service) — so command args reach shell steps without interpolation.
func runShell(dir string, env []string, script string, args []string, named map[string]string) error {
	c := exec.Command("sh", append([]string{"-c", script, "gw"}, args...)...)
	c.Dir = dir
	e := append(os.Environ(), env...)
	for k, v := range named {
		e = append(e, k+"="+v)
	}
	c.Env = e
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

// resolveUnit finds the unit a step names: exact name first, then a unique match
// on the last path segment or the directory basename.
func resolveUnit(units []workspace.Unit, ref string) (workspace.Unit, error) {
	for _, u := range units {
		if u.Name == ref {
			return u, nil
		}
	}
	var matches []workspace.Unit
	for _, u := range units {
		if lastSegment(u.Name) == ref || filepath.Base(u.Dir) == ref {
			matches = append(matches, u)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return workspace.Unit{}, failf("no unit %q in the workspace", ref).
			withHint("run `gw list` to see units")
	default:
		return workspace.Unit{}, failf("ambiguous unit %q (matches %d)", ref, len(matches)).
			withHint("use the full module path / project name to disambiguate")
	}
}

// resolveModule finds the module a step/dir names: exact path first, then a
// unique match on the last path segment or the directory basename.
func resolveModule(mods []workspace.Module, ref string) (workspace.Module, error) {
	for _, m := range mods {
		if m.Path == ref {
			return m, nil
		}
	}
	var matches []workspace.Module
	for _, m := range mods {
		if lastSegment(m.Path) == ref || filepath.Base(m.Dir) == ref {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return workspace.Module{}, failf("no module %q in the workspace", ref).
			withHint("run `gw list` to see module paths")
	default:
		return workspace.Module{}, failf("ambiguous module %q (matches %d modules)", ref, len(matches)).
			withHint("use the full module path to disambiguate (e.g. example.com/svc/api)")
	}
}

// resolveDir maps a Run dir to a module's directory (if it names one), else a
// path relative to the workspace root.
func resolveDir(root string, mods []workspace.Module, ref string) (string, error) {
	if m, err := resolveModule(mods, ref); err == nil {
		return m.Dir, nil
	}
	if filepath.IsAbs(ref) {
		return ref, nil
	}
	return filepath.Join(root, ref), nil
}

func lastSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
