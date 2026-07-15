package cmd

import (
	"encoding/json"
	"fmt"

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
			out := cmd.OutOrStdout()

			switch {
			case asDOT:
				fmt.Fprintln(out, "digraph workspace {")
				fmt.Fprintln(out, "  rankdir=LR;")
				for _, m := range g.Modules {
					fmt.Fprintf(out, "  %q;\n", m.Path)
				}
				for _, e := range g.Edges() {
					fmt.Fprintf(out, "  %q -> %q;\n", e[0], e[1])
				}
				fmt.Fprintln(out, "}")

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
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(nodes)

			default:
				for _, m := range g.Modules {
					deps := g.Dependencies(m.Path)
					if len(deps) == 0 {
						fmt.Fprintf(out, "%s\n", m.Path)
						continue
					}
					fmt.Fprintf(out, "%s\n", m.Path)
					for _, d := range deps {
						fmt.Fprintf(out, "  -> %s\n", d)
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
