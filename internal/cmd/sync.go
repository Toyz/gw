package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newSyncCmd() *cobra.Command {
	var (
		dryRun     bool
		check      bool
		noWorkSync bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Regenerate go.work's use set from discovered modules",
		Long: "sync auto-detects every module under the root and rewrites go.work's use\n" +
			"directives to match, preserving replace/godebug blocks. It then runs\n" +
			"`go work sync` to reconcile go.work.sum and the build list.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			if len(mods) == 0 {
				return fmt.Errorf("no go.mod files found under %s", root)
			}

			wf, err := workspace.ReadWorkFile(root)
			if err != nil {
				return err
			}
			if wf == nil {
				if wf, err = workspace.NewWorkFile(mods); err != nil {
					return err
				}
			}
			added, removed := workspace.SetUseSet(wf, root, mods)

			p := newPrinter(cmd)
			for _, pth := range added {
				p.printf("%s %s\n", p.s.green("+"), pth)
			}
			for _, pth := range removed {
				p.printf("%s %s\n", p.s.red("-"), pth)
			}

			if check {
				if len(added)+len(removed) > 0 {
					return failf("go.work is out of date (%d added, %d removed)", len(added), len(removed)).
						withHint("run `gw sync` to update it")
				}
				p.ok("go.work is up to date")
				return nil
			}

			if dryRun {
				p.printf("# %s (dry run)\n", workspace.WorkFileName)
				p.Out().Write(workspace.FormatWorkFile(wf))
				return nil
			}

			if err := workspace.WriteWorkFile(root, wf); err != nil {
				return err
			}
			p.ok("wrote %s: %d module(s)", workspace.WorkFileName, len(mods))

			if !noWorkSync {
				gwc := exec.Command("go", "work", "sync")
				gwc.Dir = root
				gwc.Stdout = p.Out()
				gwc.Stderr = p.Err()
				if err := gwc.Run(); err != nil {
					return fmt.Errorf("go work sync: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print go.work without writing")
	cmd.Flags().BoolVar(&check, "check", false, "exit non-zero if go.work is out of date (no write)")
	cmd.Flags().BoolVar(&noWorkSync, "no-work-sync", false, "skip running `go work sync`")
	return cmd
}
