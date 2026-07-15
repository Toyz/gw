package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newDoctorCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose workspace health (stale go.work, orphans, drift, un-hoisted replaces)",
		Long: "doctor reports problems with the workspace: a missing or stale go.work, use\n" +
			"entries with no go.mod, modules missing from go.work, replace directives still\n" +
			"in module go.mod files, and dependency/directive version mismatches. It exits\n" +
			"non-zero when any error is found (or any warning with --strict).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, cfg, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			issues := workspace.Diagnose(root, cfg, mods)
			out := cmd.OutOrStdout()

			if len(issues) == 0 {
				fmt.Fprintln(out, "ok: workspace is healthy")
				return nil
			}
			for _, is := range issues {
				fmt.Fprintf(out, "%-5s %s\n      fix: %s\n", is.Severity, is.Msg, is.Fix)
			}
			errs, warns, infos := workspace.CountBySeverity(issues)
			fmt.Fprintf(out, "\n%d error(s), %d warning(s), %d info\n", errs, warns, infos)

			if errs > 0 || (strict && warns > 0) {
				return fmt.Errorf("workspace has problems")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "also exit non-zero on warnings")
	return cmd
}
