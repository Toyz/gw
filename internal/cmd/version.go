package cmd

import "github.com/spf13/cobra"

// newVersionCmd adds `gw version` as a subcommand alongside the built-in
// `--version` flag, so both spellings work. It prints the same string cobra's
// flag would (`gw version <version>`).
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the gw version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("gw version %s\n", version)
			return nil
		},
	}
}
