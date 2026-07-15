package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newVerifyCmd() *cobra.Command {
	var (
		strict bool
		asJSON bool
		plan   bool
	)
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check that intra-workspace requires resolve to real published tags",
		Long: "verify checks the release contract that workspace mode hides. Inside a\n" +
			"workspace, a require on another member resolves to local code on disk, so\n" +
			"`go build` passes even when that version was never tagged. verify runs the\n" +
			"checks an external consumer (or a GOWORK=off release build) would hit:\n\n" +
			"  • every intra-workspace require points at a real published tag,\n" +
			"  • that tag's code still matches what's on disk (no publish drift),\n" +
			"  • no module leaks a local-path replace.\n\n" +
			"It also prints a release plan, in dependency order, for modules whose code\n" +
			"has moved past their latest tag. Exits non-zero on errors (and on warnings\n" +
			"with --strict).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			gitRoot, err := gitRootFor(root)
			if err != nil {
				return err
			}

			g := workspace.BuildGraph(mods)
			rep, err := workspace.Verify(g, mods, gitRoot)
			if err != nil {
				return err
			}

			p := newPrinter(cmd)
			if asJSON {
				return p.json(rep)
			}

			for _, f := range rep.Findings {
				p.printf("%-5s %s\n", string(f.Level), f.Message)
			}

			if plan || len(rep.Releases) > 0 {
				if len(rep.Findings) > 0 {
					p.println()
				}
				if len(rep.Releases) == 0 {
					p.println("release plan: every module is tagged and clean")
				} else {
					p.println("release plan (dependency order):")
					for _, r := range rep.Releases {
						p.printf("  %-40s %s\n", r.Module, r.Reason)
						if len(r.Dependents) > 0 {
							p.printf("  %-40s ↳ then bump + re-tag: %s\n", "", strings.Join(r.Dependents, ", "))
						}
					}
				}
			}

			errs, warns := rep.Errors(), rep.Warnings()
			switch {
			case errs > 0:
				return fmt.Errorf("%d release-contract error(s), %d warning(s)", errs, warns)
			case warns > 0 && strict:
				return fmt.Errorf("%d warning(s) (--strict)", warns)
			case warns > 0:
				p.printf("\n%d warning(s)\n", warns)
			default:
				if len(rep.Findings) == 0 {
					p.println("ok: workspace requires all resolve to published tags")
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero on warnings too")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the full report as JSON")
	cmd.Flags().BoolVar(&plan, "plan", false, "always show the release plan, even when empty")
	return cmd
}
