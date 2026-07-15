// Package cmd wires the gw cobra command tree over the workspace core.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// version is overridable at build time via -ldflags "-X ...cmd.version=...".
var version = "dev"

// rootFlag holds the -C/--root value; empty means auto-detect.
var rootFlag string

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gw",
		Short:         "gw makes Go workspaces (go.work) usable at scale",
		Long:          "gw auto-generates and maintains go.work, lints cross-module dependency\nversions, and runs commands across every module in the workspace.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.PersistentFlags().StringVarP(&rootFlag, "root", "C", "", "workspace root (default: nearest go.work, else cwd)")

	root.AddCommand(
		newInitCmd(),
		newSyncCmd(),
		newLintCmd(),
		newRunCmd(),
		newListCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newGraphCmd(),
		newAffectedCmd(),
		newDoctorCmd(),
		newExtCmd(),
	)
	for _, gc := range goCommands {
		root.AddCommand(gc.command())
	}
	return root
}

// Execute runs the gw CLI.
func Execute() {
	root := newRootCmd()
	attachExtCommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gw:", err)
		os.Exit(1)
	}
}

// resolveRoot determines the workspace root: the explicit -C flag if given,
// otherwise the nearest ancestor containing go.work, otherwise the cwd.
func resolveRoot() (string, error) {
	if rootFlag != "" {
		abs, err := filepath.Abs(rootFlag)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if root, ok := workspace.FindRoot(cwd); ok {
		return root, nil
	}
	return cwd, nil
}

// loadWorkspace resolves the root, loads config, and discovers modules.
func loadWorkspace() (root string, cfg workspace.Config, mods []workspace.Module, err error) {
	root, err = resolveRoot()
	if err != nil {
		return "", workspace.Config{}, nil, err
	}
	return loadWorkspaceAt(root)
}

// loadWorkspaceAt loads config and discovers modules for an already-resolved root.
func loadWorkspaceAt(root string) (_ string, cfg workspace.Config, mods []workspace.Module, err error) {
	cfg, err = workspace.LoadConfig(root)
	if err != nil {
		return "", workspace.Config{}, nil, fmt.Errorf("reading gw config: %w", err)
	}
	if cfg.Root != "" {
		if filepath.IsAbs(cfg.Root) {
			root = cfg.Root
		} else {
			root = filepath.Join(root, cfg.Root)
		}
	}
	mods, err = workspace.Discover(root, cfg)
	if err != nil {
		return "", workspace.Config{}, nil, err
	}
	return root, cfg, mods, nil
}
