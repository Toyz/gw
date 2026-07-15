package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/gwext"
	"github.com/toyz/gw/internal/ext"
	"github.com/toyz/gw/internal/workspace"
)

// goDirective returns the running toolchain version (e.g. "go1.26.0").
func goDirective() string { return runtime.Version() }

// toGwextModules converts discovered modules into the SDK's wire type.
func toGwextModules(mods []workspace.Module) []gwext.Module {
	out := make([]gwext.Module, 0, len(mods))
	for _, m := range mods {
		out = append(out, gwext.Module{
			Path: m.Path, Dir: m.Dir, GoVersion: m.GoVersion,
			Toolchain: m.Toolchain, Requires: m.Requires,
		})
	}
	return out
}

// fireHook runs a lifecycle hook if an extension is present. Best-effort: errors
// are surfaced to stderr but never abort the builtin command. Hooks inherit the
// workspace's configured env (config-level only; per-invocation --env flags stay
// scoped to the module commands themselves).
func fireHook(cmd *cobra.Command, root string, mods []workspace.Module, event string) {
	var env []string
	if cfg, err := workspace.LoadConfig(root); err == nil {
		if env, err = workspace.ResolveEnv(root, cfg, nil, nil); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "hook %s: env: %v\n", event, err)
			env = nil
		}
	}
	if err := ext.RunHook(root, event, toGwextModules(mods), env, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "hook %s: %v\n", event, err)
	}
}

func newExtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ext",
		Short: "Manage the workspace's .gw/build.go extension",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newExtInitCmd(), newExtBuildCmd(), newExtListCmd())
	return cmd
}

func newExtInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold .gw/build.go and its module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			dir := ext.Dir(root)
			if ext.Exists(root) {
				return fmt.Errorf("%s already exists", filepath.Join(".gw", "build.go"))
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			goVer := strings.TrimPrefix(goDirective(), "go")
			if err := os.WriteFile(filepath.Join(dir, "go.mod"),
				fmt.Appendf(nil, "module gwext.local\n\ngo %s\n", goVer), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, "build.go"), []byte(scaffoldBuildGo), 0o644); err != nil {
				return err
			}
			// Resolve the gwext dependency.
			get := exec.Command("go", "get", "github.com/toyz/gw@latest")
			get.Dir = dir
			get.Env = append(os.Environ(), "GOWORK=off")
			get.Stdout, get.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := get.Run(); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: `go get github.com/toyz/gw@latest` failed; run it in .gw manually")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scaffolded %s\nedit it, then run `gw <command>` or `gw ext list`\n",
				filepath.Join(".gw", "build.go"))
			return nil
		},
	}
}

func newExtBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Compile the extension (cached by content hash)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if !ext.Exists(root) {
				return fmt.Errorf("no .gw/build.go (run `gw ext init`)")
			}
			bin, err := ext.Build(root)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), bin)
			return nil
		},
	}
}

func newExtListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the extension's commands and hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if !ext.Exists(root) {
				return fmt.Errorf("no .gw/build.go (run `gw ext init`)")
			}
			m, err := ext.Manifest(root)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, c := range m.Commands {
				fmt.Fprintf(out, "command  %-16s %s\n", c.Name, c.Short)
			}
			for _, h := range m.Hooks {
				fmt.Fprintf(out, "hook     %s\n", h)
			}
			return nil
		},
	}
}

// attachExtCommands registers each extension command as a dynamic subcommand,
// unless a builtin already claims that name. Best-effort: on any failure it warns
// and leaves the builtin command tree intact.
func attachExtCommands(rootCmd *cobra.Command) {
	root, err := earlyResolveRoot()
	if err != nil || !ext.Exists(root) {
		return
	}
	m, err := ext.Manifest(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gw: extension: %v\n", err)
		return
	}
	builtin := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		builtin[c.Name()] = true
	}
	for _, ci := range m.Commands {
		if builtin[ci.Name] {
			fmt.Fprintf(os.Stderr, "gw: extension command %q shadowed by builtin; skipped\n", ci.Name)
			continue
		}
		name := ci.Name
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              ci.Short + " (extension)",
			DisableFlagParsing: true, // pass user flags straight through to the extension
			RunE: func(cmd *cobra.Command, args []string) error {
				// DisableFlagParsing skips the persistent -C/--root, so resolve the
				// root straight from os.Args instead of the (unparsed) rootFlag.
				r, err := earlyResolveRoot()
				if err != nil {
					return err
				}
				_, cfg, mods, err := loadWorkspaceAt(r)
				if err != nil {
					return err
				}
				env, err := workspace.ResolveEnv(r, cfg, nil, nil)
				if err != nil {
					return err
				}
				return ext.RunCommand(r, toGwextModules(mods), name, args, env, cmd.OutOrStdout(), cmd.ErrOrStderr())
			},
		})
	}
}

// earlyResolveRoot mirrors resolveRoot but reads -C/--root straight from os.Args,
// since cobra has not parsed flags yet at registration time.
func earlyResolveRoot() (string, error) {
	if v := scanRootFlag(os.Args[1:]); v != "" {
		return filepath.Abs(v)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if r, ok := workspace.FindRoot(cwd); ok {
		return r, nil
	}
	return cwd, nil
}

func scanRootFlag(args []string) string {
	for i, a := range args {
		switch {
		case a == "-C" || a == "--root":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--root="):
			return strings.TrimPrefix(a, "--root=")
		case strings.HasPrefix(a, "-C="):
			return strings.TrimPrefix(a, "-C=")
		}
	}
	return ""
}

const scaffoldBuildGo = `package main

import (
	"fmt"

	"github.com/toyz/gw/gwext"
)

func main() {
	// Custom command: invoke as ` + "`gw hello`" + `.
	gwext.Command("hello", "example custom command", func(c *gwext.Context) error {
		fmt.Printf("hello from %d module(s) at %s\n", len(c.Modules), c.Root)
		return nil
	})

	// Lifecycle hook: runs after ` + "`gw sync`" + `.
	gwext.Hook("post-sync", func(c *gwext.Context) error {
		fmt.Println("post-sync: go.work updated")
		return nil
	})

	gwext.Main()
}
`
