package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newAffectedCmd() *cobra.Command {
	var (
		since        string
		seedsOnly    bool
		asDirs       bool
		asJSON       bool
		servicesOnly bool
	)
	cmd := &cobra.Command{
		Use:   "affected --since <ref>",
		Short: "List modules impacted by changes since a git ref",
		Long: "affected diffs the working tree against a git ref, maps changed files to their\n" +
			"owning modules, and walks the dependency graph to every module that must be\n" +
			"rebuilt/retested. Feed it to selective CI, e.g. `gw affected --since main`.\n" +
			"With --services it instead lists the [services.<name>] a diff touches (by\n" +
			"directory) — for change-based redeploy across languages in a polyglot repo.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if since == "" {
				return failf("--since <ref> is required").
					withHint("e.g. --since main or --since HEAD~1")
			}
			root, cfg, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			gitRoot, err := gitRootFor(root)
			if err != nil {
				return err
			}
			changed, err := workspace.ChangedFiles(gitRoot, since)
			if err != nil {
				return fmt.Errorf("git diff against %q: %w", since, err)
			}

			g := workspace.BuildGraph(mods)
			seeds, impacted := workspace.AffectedModules(g, mods, changed)
			services := workspace.AffectedServices(root, cfg.Services, changed)

			p := newPrinter(cmd)

			if asJSON {
				return p.json(map[string]any{"seeds": seeds, "impacted": impacted, "services": services})
			}

			// --services switches text output to the affected service names, so it
			// pipes straight into a deploy step (independent of the Go module set).
			if servicesOnly {
				for _, s := range services {
					p.println(s)
				}
				return nil
			}

			result := impacted
			if seedsOnly {
				result = seeds
			}
			for _, mp := range result {
				if asDirs {
					p.println(workspace.UsePath(root, g.Module(mp).Dir))
					continue
				}
				p.println(mp)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "git ref to diff against (e.g. main, HEAD~1)")
	cmd.Flags().BoolVar(&seedsOnly, "seeds", false, "only directly-changed modules (skip dependents)")
	cmd.Flags().BoolVar(&asDirs, "dir", false, "print module use-paths instead of module paths")
	cmd.Flags().BoolVar(&servicesOnly, "services", false, "list affected [services.<name>] instead of modules")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON {seeds, impacted, services}")
	return cmd
}
