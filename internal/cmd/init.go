package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newInitCmd() *cobra.Command {
	var (
		force  bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a go.work from existing modules and hoist replace directives",
		Long: "init scans the workspace root for modules, creates go.work, and moves every\n" +
			"replace directive out of each module's go.mod up into go.work. It refuses to\n" +
			"overwrite an existing go.work unless --force is given.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			root, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			if len(mods) == 0 {
				return fmt.Errorf("no go.mod files found under %s", root)
			}
			if workspace.WorkFileExists(root) && !force && !dryRun {
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
			p.printf("wrote %s: %d module(s), hoisted replaces from %d go.mod file(s)\n",
				workspace.WorkFileName, len(mods), len(mutated))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing go.work")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the result without writing anything")
	return cmd
}
