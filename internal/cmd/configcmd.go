package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// stepVerbs maps a config step verb to the argv run in a module's directory.
var stepVerbs = map[string][]string{
	"build":    {"go", "build", "./..."},
	"test":     {"go", "test", "./..."},
	"vet":      {"go", "vet", "./..."},
	"generate": {"go", "generate", "./..."},
	"tidy":     {"go", "mod", "tidy"},
	"run":      {"go", "run", "."},
}

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
		rootCmd.AddCommand(&cobra.Command{
			Use:   name,
			Short: short + " (config)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return runConfigCommand(newPrinter(cmd), cc)
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

// runConfigCommand loads the workspace and executes a config command end to end.
func runConfigCommand(p *printer, cc workspace.ConfigCommand) error {
	root, cfg, mods, err := loadWorkspaceEarly()
	if err != nil {
		return err
	}
	env, err := workspace.ResolveConfigEnv(root, cfg)
	if err != nil {
		return err
	}
	return execConfigCommand(p, root, mods, env, cc)
}

// execConfigCommand runs a command/hook body: each step in order. A step is
// either a "<module>:<verb>" op (run in the module's directory) or a shell
// command (run in the command's dir, else the root). Shared by commands + hooks.
func execConfigCommand(p *printer, root string, mods []workspace.Module, env []string, cc workspace.ConfigCommand) error {
	shellDir := root
	if cc.Dir != "" {
		d, err := resolveDir(root, mods, cc.Dir)
		if err != nil {
			return err
		}
		shellDir = d
	}
	for _, step := range cc.Steps {
		if modRef, verb, isModule := parseStep(step); isModule {
			if err := runModuleStep(p, mods, env, modRef, verb); err != nil {
				return err
			}
			continue
		}
		p.step("%s", step)
		if err := runShell(shellDir, env, step); err != nil {
			e := failf("`%s` failed: %v", step, err)
			if cc.Dir == "" && looksMisplacedGoCmd(step) {
				// Bare `go build/generate/... ./...` (or `go mod tidy`) run from the
				// workspace root fails — the root has no go.mod. Point at the fix.
				e = e.withHint(`go tools are module-relative — use a "<module>:<verb>" step (e.g. api:generate), or set dir`)
			}
			return e
		}
	}
	return nil
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

// parseStep classifies a step: a whitespace-free "<module>:<verb>" whose verb is
// one of stepVerbs is a module op; anything else is a shell command.
func parseStep(step string) (modRef, verb string, isModule bool) {
	if strings.ContainsAny(step, " \t") {
		return "", "", false
	}
	modRef, verb, ok := strings.Cut(step, ":")
	if !ok || modRef == "" {
		return "", "", false
	}
	if _, known := stepVerbs[verb]; !known {
		return "", "", false
	}
	return modRef, verb, true
}

// runModuleStep runs a "<module>:<verb>" op in the resolved module's directory.
func runModuleStep(p *printer, mods []workspace.Module, env []string, modRef, verb string) error {
	m, err := resolveModule(mods, modRef)
	if err != nil {
		return err
	}
	job := workspace.Job{Module: m, Argv: stepVerbs[verb], Env: env}
	results := workspace.RunAcross(context.Background(), []workspace.Job{job}, workspace.ExecOpts{
		Stdout: p.Out(),
		Stderr: p.Err(),
		Header: func(mod string) string {
			return p.s.cyan("→") + " " + p.s.bold(mod) + " " + p.s.dim(verb)
		},
	})
	if workspace.WorstExit(results) != 0 {
		return failf("step %q failed", modRef+":"+verb)
	}
	return nil
}

// runShell runs a shell script (sh -c) in dir with the workspace env layered on.
func runShell(dir string, env []string, script string) error {
	c := exec.Command("sh", "-c", script)
	c.Dir = dir
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
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
