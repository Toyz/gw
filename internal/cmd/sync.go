package cmd

import (
	"fmt"
	"os"
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

			out := cmd.OutOrStdout()
			if !check && !dryRun {
				fireHook(cmd, root, mods, "pre-sync")
			}
			for _, p := range added {
				fmt.Fprintf(out, "+ %s\n", p)
			}
			for _, p := range removed {
				fmt.Fprintf(out, "- %s\n", p)
			}

			if check {
				if len(added)+len(removed) > 0 {
					return fmt.Errorf("go.work is out of date (%d added, %d removed)", len(added), len(removed))
				}
				fmt.Fprintln(out, "go.work is up to date")
				return nil
			}

			if dryRun {
				fmt.Fprintf(out, "# %s (dry run)\n", workspace.WorkFileName)
				out.Write(workspace.FormatWorkFile(wf))
				return nil
			}

			if err := workspace.WriteWorkFile(root, wf); err != nil {
				return err
			}
			fmt.Fprintf(out, "wrote %s: %d module(s)\n", workspace.WorkFileName, len(mods))

			if !noWorkSync {
				gwc := exec.Command("go", "work", "sync")
				gwc.Dir = root
				gwc.Stdout = os.Stdout
				gwc.Stderr = os.Stderr
				if err := gwc.Run(); err != nil {
					return fmt.Errorf("go work sync: %w", err)
				}
			}
			fireHook(cmd, root, mods, "post-sync")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print go.work without writing")
	cmd.Flags().BoolVar(&check, "check", false, "exit non-zero if go.work is out of date (no write)")
	cmd.Flags().BoolVar(&noWorkSync, "no-work-sync", false, "skip running `go work sync`")
	return cmd
}
