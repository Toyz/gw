package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newGraphCmd() *cobra.Command {
	var (
		asDOT  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Print the intra-workspace module dependency graph",
		Long:  "graph emits the DAG of workspace modules (edge A->B means A requires B).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _, mods, err := loadWorkspace()
			if err != nil {
				return err
			}
			g := workspace.BuildGraph(mods)
			p := newPrinter(cmd)

			switch {
			case asDOT:
				p.println("digraph workspace {")
				p.println("  rankdir=LR;")
				for _, m := range g.Modules {
					p.printf("  %q;\n", m.Path)
				}
				for _, e := range g.Edges() {
					p.printf("  %q -> %q;\n", e[0], e[1])
				}
				p.println("}")

			case asJSON:
				type node struct {
					Module     string   `json:"module"`
					DependsOn  []string `json:"dependsOn,omitempty"`
					Dependents []string `json:"dependents,omitempty"`
				}
				nodes := make([]node, 0, len(g.Modules))
				for _, m := range g.Modules {
					nodes = append(nodes, node{
						Module:     m.Path,
						DependsOn:  g.Dependencies(m.Path),
						Dependents: g.Dependents(m.Path),
					})
				}
				enc := json.NewEncoder(p.Out())
				enc.SetIndent("", "  ")
				return enc.Encode(nodes)

			default:
				for _, m := range g.Modules {
					deps := g.Dependencies(m.Path)
					if len(deps) == 0 {
						p.printf("%s\n", m.Path)
						continue
					}
					p.printf("%s\n", m.Path)
					for _, d := range deps {
						p.printf("  -> %s\n", d)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asDOT, "dot", false, "emit Graphviz DOT")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}
