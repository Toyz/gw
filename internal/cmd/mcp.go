package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/mcp"
)

func newMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run gw as a Model Context Protocol (MCP) stdio server",
		Long: "mcp exposes the workspace to an AI agent over MCP (stdio): list modules,\n" +
			"lint version drift, map the dependency graph, compute the change-affected\n" +
			"set, sync go.work, and run tests. Register it with:\n" +
			"  claude mcp add gw -- gw mcp",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcp.Serve(os.Stdin, os.Stdout, version)
		},
	}
}
