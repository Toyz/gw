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
		// Every workspace command fires pre-/post-<name> hooks from one place —
		// builtins and custom extension commands alike. Cheap when nothing hooks
		// the event (fireHook gates on the manifest). post- fires on success.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if hookable(cmd, args) {
				fireHook(cmd, "pre-"+cmd.Name())
			}
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if hookable(cmd, args) {
				fireHook(cmd, "post-"+cmd.Name())
			}
			return nil
		},
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
		newConfigCmd(),
		newExtCmd(),
	)
	for _, gc := range goCommands {
		root.AddCommand(gc.command())
	}
	return root
}

// hookCommandSkip lists commands that never fire pre-/post- hooks: cobra's meta
// verbs and the management groups whose subcommands aren't workspace runs (`gw
// ext build` is not `gw build`; `gw config init` scaffolds a file).
var hookCommandSkip = map[string]bool{
	"help":             true,
	"completion":       true,
	"__complete":       true,
	"__completeNoDesc": true,
	"ext":              true,
	"config":           true,
}

// hookSuppressFlags mark a read-only/preview run: with one set, the command
// makes no changes, so its (possibly side-effecting) hooks shouldn't fire.
var hookSuppressFlags = []string{"dry-run", "check"}

// hookable reports whether cmd is a direct workspace subcommand of the root that
// should fire pre-/post- hooks — skipping meta verbs, the ext subtree (whose
// children have "ext", not "gw", as their parent), preview runs, and help
// queries. args is the command's raw argv: DisableFlagParsing commands (go
// passthrough, custom/override) reach here with -h/--help unparsed and render
// help from their own RunE, so a hook must not fire around a help request.
func hookable(cmd *cobra.Command, args []string) bool {
	p := cmd.Parent()
	if p == nil || p.Name() != "gw" || hookCommandSkip[cmd.Name()] || wantsHelp(args) {
		return false
	}
	for _, name := range hookSuppressFlags {
		if f := cmd.Flags().Lookup(name); f != nil && f.Value.String() == "true" {
			return false
		}
	}
	return true
}

// Execute runs the gw CLI.
func Execute() {
	root := newRootCmd()
	if extEnabled() {
		attachExtCommands(root)
		attachConfigCommands(root)
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

// loadWorkspaceEarly is loadWorkspace for DisableFlagParsing commands (go
// passthrough, custom/override, and the hooks that wrap them): it reads -C
// straight from os.Args, since the parsed rootFlag is never populated for them.
func loadWorkspaceEarly() (root string, cfg workspace.Config, mods []workspace.Module, err error) {
	root, err = earlyResolveRoot()
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
