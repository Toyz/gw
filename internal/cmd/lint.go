package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newLintCmd() *cobra.Command {
	var (
		fix      bool
		strategy string
	)
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Report cross-module dependency and go-directive version mismatches",
		Long: "lint finds dependencies required at different versions across modules, plus\n" +
			"mismatched go/toolchain directives. It exits non-zero when mismatches remain.\n" +
			"--fix aligns dependency versions (go/toolchain are reported, never auto-changed).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, cfg, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			mismatches := workspace.Lint(mods)
			out := cmd.OutOrStdout()

			if len(mismatches) == 0 {
				fmt.Fprintln(out, "ok: no version mismatches")
				return nil
			}

			for _, mm := range mismatches {
				label := mm.Dep
				if mm.Dep == workspace.GoDirective || mm.Dep == workspace.ToolchainDirective {
					label = "[" + mm.Dep + " directive]"
				}
				fmt.Fprintf(out, "%s:\n", label)
				for _, v := range mm.SortedVersions() {
					fmt.Fprintf(out, "  %-20s %s\n", v, strings.Join(mm.Versions[v], ", "))
				}
			}

			if !fix {
				return fmt.Errorf("%d version mismatch(es) found", len(mismatches))
			}

			strat := workspace.Strategy(strategy)
			if strat != workspace.Highest && strat != workspace.Lowest {
				return fmt.Errorf("invalid --strategy %q (want highest|lowest)", strategy)
			}
			changed := workspace.Fix(mods, mismatches, strat, cfg.Pins)
			for _, m := range changed {
				if err := m.Save(); err != nil {
					return fmt.Errorf("rewriting %s go.mod: %w", m.Path, err)
				}
			}
			fmt.Fprintf(out, "\nfixed dependency versions in %d module(s) (strategy: %s)\n", len(changed), strat)

			// Directive mismatches are not auto-fixed; surface if any remain.
			for _, mm := range mismatches {
				if mm.Dep == workspace.GoDirective || mm.Dep == workspace.ToolchainDirective {
					fmt.Fprintf(out, "note: %s directive still mismatched (align manually)\n", mm.Dep)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "rewrite go.mod files to align dependency versions")
	cmd.Flags().StringVar(&strategy, "strategy", "highest", "version to converge on: highest|lowest")
	return cmd
}
