package cmd

import (
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newDocCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc [unit] [symbol]",
		Short: "Show documentation for a workspace unit",
		Long: "doc resolves a workspace unit and shows its documentation:\n" +
			"  • a Go module -> `go doc <import-path> [symbol]`, with go's own flags\n" +
			"    (-all, -src, -u, -c) forwarded — resolved through go.work from any dir\n" +
			"  • a project -> its toolchain's `doc` task (e.g. rust -> `cargo doc`)\n" +
			"The unit is a short name (`api`), full path, or `<unit>/<subpkg>`. With no\n" +
			"unit, it lists what's documentable.",
		// go doc has its own flags to forward, and -C isn't parsed under
		// DisableFlagParsing — so resolve the root from os.Args like the go verbs.
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, rawArgs []string) error {
			if wantsHelp(rawArgs) {
				return cmd.Help()
			}
			root, cfg, mods, err := loadWorkspaceEarly()
			if err != nil {
				return err
			}
			units, _ := workspace.Units(root, mods, cfg.Projects)
			p := newPrinter(cmd)

			// Leading flags are go doc's; the first positional is the unit ref.
			args := stripRootFlag(rawArgs)
			i := 0
			for i < len(args) && strings.HasPrefix(args[i], "-") {
				i++
			}
			flags, pos := args[:i], args[i:]
			if len(pos) == 0 {
				return listDocUnits(p, units)
			}
			ref, symbol := pos[0], pos[1:]

			// 1. ref is a unit: a Go module docs via go doc, a project via its task.
			u, unitErr := resolveUnit(units, ref)
			if unitErr == nil {
				if u.IsModule {
					return runGoDoc(p, root, flags, u.Name, symbol)
				}
				return runProjectDoc(p, cfg, u)
			}
			// 2. "<unit-short>/<subpkg>": resolve the head to a Go module's path.
			if head, tail, ok := strings.Cut(ref, "/"); ok {
				if hu, err := resolveUnit(units, head); err == nil && hu.IsModule {
					return runGoDoc(p, root, flags, hu.Name+"/"+tail, symbol)
				}
			}
			// 3. A path-like ref (full import path, external dep) goes straight to
			// go doc; a bare name that matched no unit gets the resolve error+hint.
			if strings.ContainsAny(ref, "./") {
				return runGoDoc(p, root, flags, ref, symbol)
			}
			return unitErr
		},
	}
}

// runGoDoc runs `go doc [flags] <pkg> [symbol]` from the workspace root, where
// go.work resolves any workspace package. Its output is the doc lookup verbatim.
func runGoDoc(p *printer, root string, flags []string, pkg string, symbol []string) error {
	argv := append([]string{"doc"}, flags...)
	argv = append(argv, pkg)
	argv = append(argv, symbol...)
	c := exec.Command("go", argv...)
	c.Dir = root
	return stream(p, c)
}

// runProjectDoc runs a non-Go unit's `doc` task in its directory (e.g. cargo doc,
// or a [toolchains.<tc>] doc / per-project override).
func runProjectDoc(p *printer, cfg workspace.Config, u workspace.Unit) error {
	argv, shell, err := workspace.TaskCommand(cfg, u, "doc")
	if err != nil {
		return failf("%s: %v", u.Name, err)
	}
	p.step("%s %s doc", u.Name, u.Toolchain)
	var c *exec.Cmd
	if shell != "" {
		c = exec.Command("sh", "-c", shell)
	} else {
		c = exec.Command(argv[0], argv[1:]...)
	}
	c.Dir = u.Dir
	return stream(p, c)
}

// listDocUnits prints what's documentable when `gw doc` gets no unit — turning
// the "no package at the workspace root" dead-end into a menu.
func listDocUnits(p *printer, units []workspace.Unit) error {
	if len(units) == 0 {
		return failf("no units in the workspace").
			withHint("run `gw init`, or add [projects] to gw.toml")
	}
	p.info("documentable units:")
	for _, u := range units {
		kind := u.Toolchain
		if u.IsModule {
			kind = "go module"
		}
		p.printf("  %-30s %s\n", u.Name, p.s.dim(kind))
	}
	p.hint("gw doc <unit> [symbol]")
	return nil
}

// stream runs c with its output on the printer's writers, propagating the child's
// exit code without wrapping (the tool already printed why it failed).
func stream(p *printer, c *exec.Cmd) error {
	c.Stdout, c.Stderr = p.Out(), p.Err()
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err // couldn't start the tool (e.g. not installed)
	}
	return nil
}
