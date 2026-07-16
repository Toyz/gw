// Package cmd wires the gw cobra command tree over the workspace core.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// version is overridable at build time via -ldflags "-X ...cmd.version=...".
var version = "dev"

func init() {
	// Report the module version from build info: an exact tag for
	// `go install ...@v1.2.3`, or a VCS pseudo-version (commit + dirty) for an
	// in-repo build. Falls back to "dev" only when no build info is available.
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
}

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
		newVerifyCmd(),
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
	if extEnabled() {
		attachExtCommands(root)
	}
	if err := root.Execute(); err != nil {
		renderError(os.Stderr, err)
		os.Exit(1)
	}
}

// renderError prints a command failure uniformly: "✗ <msg>", plus a dim
// "help: <hint>" line when the error carries one (a *cmdError).
func renderError(w io.Writer, err error) {
	s := newStyler(w)
	fmt.Fprintf(w, "%s %s\n", s.red("✗"), err.Error())
	var ce *cmdError
	if errors.As(err, &ce) && ce.hint != "" {
		fmt.Fprintf(w, "  %s %s\n", s.dim("help:"), s.dim(ce.hint))
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

// gitRootFor resolves the repository top level for the workspace root, wrapping
// the "no git" case in a consistent error. Used by commands that diff or tag.
func gitRootFor(root string) (string, error) {
	gitRoot, err := workspace.GitRoot(root)
	if err != nil {
		return "", failf("not a git repository (or git unavailable)").
			withHint("run `git init` in the workspace root")
	}
	return gitRoot, nil
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
