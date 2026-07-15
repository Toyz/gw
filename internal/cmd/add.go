package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a module directory to go.work",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			dir := args[0]
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(root, dir)
			}
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
				return fmt.Errorf("no go.mod at %s", dir)
			}
			mods, err := workspace.Discover(dir, workspace.Config{})
			if err != nil {
				return err
			}
			modPath := ""
			for _, m := range mods {
				if m.Dir == dir {
					modPath = m.Path
					break
				}
			}

			wf, err := workspace.ReadWorkFile(root)
			if err != nil {
				return err
			}
			if wf == nil {
				if wf, err = workspace.NewWorkFile(mods); err != nil {
					return err
				}
			}
			up := workspace.UsePath(root, dir)
			if err := wf.AddUse(up, modPath); err != nil {
				return err
			}
			if err := workspace.WriteWorkFile(root, wf); err != nil {
				return err
			}
			p.printf("added %s (%s)\n", up, modPath)
			return nil
		},
	}
	return cmd
}
