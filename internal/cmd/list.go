package cmd

import (
	"sort"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newListCmd() *cobra.Command {
	var (
		verbose bool
		asJSON  bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace modules (and, with -v, their external requires)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			p := newPrinter(cmd)

			if asJSON {
				type modJSON struct {
					Path      string            `json:"path"`
					Dir       string            `json:"dir"`
					Use       string            `json:"use"`
					GoVersion string            `json:"go,omitempty"`
					Toolchain string            `json:"toolchain,omitempty"`
					Requires  map[string]string `json:"requires,omitempty"`
				}
				list := make([]modJSON, 0, len(mods))
				for _, m := range mods {
					list = append(list, modJSON{
						Path: m.Path, Dir: m.Dir, Use: workspace.UsePath(root, m.Dir),
						GoVersion: m.GoVersion, Toolchain: m.Toolchain, Requires: m.Requires,
					})
				}
				return p.json(list)
			}

			for _, m := range mods {
				p.printf("%s\t%s\n", workspace.UsePath(root, m.Dir), m.Path)
				if verbose {
					if m.GoVersion != "" {
						p.printf("    go %s\n", m.GoVersion)
					}
					deps := make([]string, 0, len(m.Requires))
					for d := range m.Requires {
						deps = append(deps, d)
					}
					sort.Strings(deps)
					for _, d := range deps {
						p.printf("    %s %s\n", d, m.Requires[d])
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show each module's go version and external requires")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}
