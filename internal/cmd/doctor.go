package cmd

import (
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
			findings := make([]finding, len(issues))
			for i, is := range issues {
				findings[i] = finding{
					level: severityLevel(is.Severity),
					msg:   is.Msg,
					hint:  is.Fix,
				}
			}
			return newPrinter(cmd).report(findings, strict, "workspace is healthy")
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "also exit non-zero on warnings")
	return cmd
}

// severityLevel maps a workspace diagnosis severity to a printer level.
func severityLevel(s workspace.Severity) level {
	switch s {
	case workspace.SevError:
		return lvlError
	case workspace.SevWarn:
		return lvlWarn
	default:
		return lvlInfo
	}
}
