package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <path>",
		Aliases: []string{"rm"},
		Short:   "Remove a module directory from go.work",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			wf, err := workspace.ReadWorkFile(root)
			if err != nil {
				return err
			}
			if wf == nil {
				return failf("no %s at %s", workspace.WorkFileName, root).
					withHint("run `gw sync` or `gw init` first")
			}

			dir := args[0]
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(root, dir)
			}
			canonical := workspace.UsePath(root, dir)

			// Accept several spellings of the same directory.
			candidates := []string{canonical, args[0], "./" + filepath.ToSlash(args[0]), filepath.ToSlash(args[0])}
			found := false
			for _, u := range wf.Use {
				for _, c := range candidates {
					if u.Path == c {
						if err := wf.DropUse(u.Path); err != nil {
							return err
						}
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				return failf("%s is not a use directory in go.work", args[0]).
					withHint("run `gw list` to see the workspace's modules")
			}
			if err := workspace.WriteWorkFile(root, wf); err != nil {
				return err
			}
			p.ok("removed %s", canonical)
			return nil
		},
	}
	return cmd
}
