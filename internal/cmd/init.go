package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/ext"
	"github.com/toyz/gw/internal/workspace"
)

func newInitCmd() *cobra.Command {
	var (
		force   bool
		dryRun  bool
		all     bool
		withCfg bool
		withExt bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a go.work from existing modules and hoist replace directives",
		Long: "init scans the workspace root for modules, creates go.work, and moves every\n" +
			"replace directive out of each module's go.mod up into go.work. It refuses to\n" +
			"overwrite an existing go.work unless --force is given.\n\n" +
			"--config also scaffolds gw.toml, --ext also scaffolds .gw/build.go, and --all\n" +
			"does both — a one-shot bootstrap. Those steps skip (with a note) whatever\n" +
			"already exists, so `gw init --all` is safe to re-run.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			root, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			wantCfg := all || withCfg
			wantExt := all || withExt

			if err := initWorkspace(p, root, mods, force, dryRun, wantCfg || wantExt); err != nil {
				return err
			}
			// Extra scaffolds don't run under --dry-run (they only write files).
			if dryRun {
				return nil
			}
			if wantCfg {
				if existing, ok := workspace.ConfigPath(root); ok {
					p.warnf("%s already exists; skipped", filepath.Base(existing))
				} else if err := writeStarterConfig(root); err != nil {
					return err
				} else {
					p.ok("wrote %s", workspace.DefaultConfigName)
				}
			}
			if wantExt {
				if ext.Exists(root) {
					p.warnf("%s already exists; skipped", filepath.Join(".gw", "build.go"))
				} else if err := scaffoldExtInto(cmd, p, root); err != nil {
					return err
				} else {
					p.ok("scaffolded %s", filepath.Join(".gw", "build.go"))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing go.work")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the go.work result without writing anything")
	cmd.Flags().BoolVar(&all, "all", false, "also scaffold gw.toml and .gw/build.go (= --config --ext)")
	cmd.Flags().BoolVar(&withCfg, "config", false, "also scaffold a starter gw.toml")
	cmd.Flags().BoolVar(&withExt, "ext", false, "also scaffold a .gw/build.go extension")
	return cmd
}

// initWorkspace creates/refreshes go.work and hoists replaces. When the go.work
// already exists and no --force: it errors on a plain init, but if other scaffold
// steps were requested (extras) it warns and skips the workspace step instead.
func initWorkspace(p *printer, root string, mods []workspace.Module, force, dryRun, extras bool) error {
	if len(mods) == 0 {
		if extras {
			p.warnf("no go.mod files under %s; skipping go.work", root)
			return nil
		}
		return failf("no go.mod files found under %s", root).
			withHint("add a module, or run in a repo that has one")
	}
	if workspace.WorkFileExists(root) && !force && !dryRun {
		if extras {
			p.warnf("%s already exists; skipped (use --force to regenerate)", workspace.WorkFileName)
			return nil
		}
		return failf("%s already exists", workspace.WorkFileName).
			withHint("re-run with --force to regenerate")
	}

	wf, err := workspace.NewWorkFile(mods)
	if err != nil {
		return err
	}
	workspace.SetUseSet(wf, root, mods)
	mutated, warnings := workspace.HoistReplaces(root, wf, mods)
	for _, w := range warnings {
		p.warnf("%s", w)
	}

	if dryRun {
		p.printf("# %s (dry run)\n", workspace.WorkFileName)
		p.Out().Write(workspace.FormatWorkFile(wf))
		p.printf("\n%d module(s), %d go.mod file(s) would change:\n", len(mods), len(mutated))
		for _, m := range mutated {
			p.printf("  %s\n", m.Path)
		}
		return nil
	}

	if err := workspace.WriteWorkFile(root, wf); err != nil {
		return err
	}
	for _, m := range mutated {
		if err := m.Save(); err != nil {
			return fmt.Errorf("rewriting %s go.mod: %w", m.Path, err)
		}
	}
	p.ok("wrote %s: %d module(s), hoisted replaces from %d go.mod file(s)",
		workspace.WorkFileName, len(mods), len(mutated))
	return nil
}
