package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/gwext"
	"github.com/toyz/gw/internal/ext"
	"github.com/toyz/gw/internal/workspace"
)

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

// extEnabled reports whether extensions are active for this process. A call-
// through child (spawned by c.Builtin with GW_SKIP_EXT=1) runs the pure builtin
// tree: no extension commands, no overrides, no hooks. This both prevents an
// override from recursing into itself and keeps fall-through behavior identical
// to the un-extended builtin.
func extEnabled() bool { return os.Getenv("GW_SKIP_EXT") != "1" }

// registeredHooks is the set of hook events the workspace's extension declares,
// cached from its manifest at startup (see attachExtCommands). fireHook consults
// it so an event with no hook — e.g. pre-list when nothing hooks list — never
// builds or spawns the extension.
var registeredHooks map[string]bool

// fireHook runs the extension's hook for event, if one is registered. It is
// best-effort: errors go to stderr but never abort the command. Gated on the
// cached manifest, so unregistered events cost nothing. It loads the workspace
// itself; a load failure (e.g. before `gw init` writes go.work) silently skips.
// Hooks inherit the workspace's configured env (config-level only; per-
// invocation --env flags stay scoped to the module commands themselves).
//
// Root resolution reads -C straight from os.Args (earlyResolveRoot) rather than
// the parsed rootFlag: hooks fire from the root's persistent pre/post-run, and
// the commands they wrap (go passthrough, custom/override) use DisableFlagParsing,
// so rootFlag is never populated for them.
func fireHook(cmd *cobra.Command, event string) {
	if !registeredHooks[event] {
		return
	}
	root, cfg, mods, err := loadWorkspaceEarly()
	if err != nil {
		return
	}
	p := newPrinter(cmd)
	env, err := workspace.ResolveEnv(root, cfg, nil, nil)
	if err != nil {
		p.warnf("hook %s: env: %v", event, err)
		env = nil
	}
	if err := ext.RunHook(root, event, toGwextModules(mods), env, p.Out(), p.Err()); err != nil {
		p.warnf("hook %s: %v", event, err)
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
			p := newPrinter(cmd)
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			dir := ext.Dir(root)
			if ext.Exists(root) {
				return failf("%s already exists", filepath.Join(".gw", "build.go"))
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			goVer := workspace.UserGoVersion()
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
				p.warnf("note: `go get github.com/toyz/gw@latest` failed; run it in .gw manually")
			}
			p.ok("scaffolded %s", filepath.Join(".gw", "build.go"))
			p.info("edit it, then run `gw <command>` or `gw ext list`")
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
				return failf("no .gw/build.go extension").withHint("run `gw ext init`")
			}
			bin, err := ext.Build(root)
			if err != nil {
				return err
			}
			newPrinter(cmd).println(bin)
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
				return failf("no .gw/build.go extension").withHint("run `gw ext init`")
			}
			m, err := ext.Manifest(root)
			if err != nil {
				return err
			}
			p := newPrinter(cmd)
			for _, c := range m.Commands {
				kind := "command"
				if c.Override {
					kind = "override"
				}
				short := c.Short
				if c.Passthrough {
					short += " (passthrough)"
				}
				p.printf("%s  %-16s %s\n", kind, c.Name, short)
				for _, fl := range c.Flags {
					def := ""
					if fl.Def != "" && fl.Def != "false" {
						def = " [" + fl.Def + "]"
					}
					names := "--" + fl.Name
					for _, a := range fl.Aliases {
						names += ", -" + a
					}
					p.printf("           %s  %s%s\n", names, fl.Help, def)
				}
			}
			for _, h := range m.Hooks {
				p.printf("hook     %s\n", h)
			}
			if m.Providers > 0 {
				p.printf("provider %d build provider(s)\n", m.Providers)
			}
			for _, h := range m.Hidden {
				p.printf("hides    %s\n", h)
			}
			return nil
		},
	}
}

// attachExtCommands wires an extension's commands into the tree. It first hides
// any builtins the extension asks to hide, then adds each extension command —
// replacing a colliding builtin only when the command is an explicit Override,
// otherwise skipping it. Best-effort: on any failure it warns and leaves the
// builtin command tree intact.
func attachExtCommands(rootCmd *cobra.Command) {
	root, err := earlyResolveRoot()
	if err != nil || !ext.Exists(root) {
		return
	}
	p := newPrinter(rootCmd)
	m, err := ext.Manifest(root)
	if err != nil {
		p.warnf("extension: %v", err)
		return
	}
	// Cache the declared hook events so fireHook can gate on them without
	// re-reading (or re-spawning) the extension per command.
	registeredHooks = make(map[string]bool, len(m.Hooks))
	for _, h := range m.Hooks {
		registeredHooks[h] = true
	}
	builtin := map[string]*cobra.Command{}
	for _, c := range rootCmd.Commands() {
		builtin[c.Name()] = c
	}
	for _, name := range m.Hidden {
		if c, ok := builtin[name]; ok {
			rootCmd.RemoveCommand(c)
			delete(builtin, name)
		}
	}
	for _, ci := range m.Commands {
		if c, clash := builtin[ci.Name]; clash {
			if !ci.Override {
				p.warnf("extension command %q shadowed by builtin; skipped (use gwext.Override to replace)", ci.Name)
				continue
			}
			rootCmd.RemoveCommand(c)
			delete(builtin, ci.Name)
		}
		name := ci.Name
		// Surface the override so it is never silent: help and `ext list` both
		// announce that this verb is extended/overridden in this workspace.
		suffix := " (extension)"
		if ci.Override {
			suffix = " (overrides builtin)"
		}
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              ci.Short + suffix,
			DisableFlagParsing: true, // pass user flags straight through to the extension
			RunE: func(cmd *cobra.Command, args []string) error {
				// DisableFlagParsing skips the persistent -C/--root, so this
				// resolves the root straight from os.Args (loadWorkspaceEarly).
				r, cfg, mods, err := loadWorkspaceEarly()
				if err != nil {
					return err
				}
				env, _, err := workspaceEnv(r, cfg, mods)
				if err != nil {
					return err
				}
				return ext.RunCommand(r, toGwextModules(mods), name, stripRootFlag(args), env, cmd.OutOrStdout(), cmd.ErrOrStderr())
			},
		})
	}
}

// stripRootFlag removes the persistent -C/--root flag (and its value) from an
// extension command's args, which cobra leaves in place under DisableFlagParsing.
// The extension parses the rest (its own flags) however it likes.
func stripRootFlag(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if _, span, ok := matchRootFlag(args, i); ok {
			i += span - 1 // skip the flag (and its value, if separate)
			continue
		}
		out = append(out, args[i])
	}
	return out
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
	for i := range args {
		if v, _, ok := matchRootFlag(args, i); ok {
			return v
		}
	}
	return ""
}

// matchRootFlag reports whether args[i] begins the persistent -C/--root flag,
// returning its value and how many args it spans: 2 for "-C value", 1 for the
// joined "-C=value"/"--root=value" or a dangling "-C" with no value. It backs
// the three places that must handle -C under DisableFlagParsing (splitExecArgs
// consumes it, stripRootFlag drops it, scanRootFlag reads it).
func matchRootFlag(args []string, i int) (value string, span int, ok bool) {
	switch a := args[i]; {
	case a == "-C" || a == "--root":
		if i+1 < len(args) {
			return args[i+1], 2, true
		}
		return "", 1, true
	case strings.HasPrefix(a, "-C="):
		return strings.TrimPrefix(a, "-C="), 1, true
	case strings.HasPrefix(a, "--root="):
		return strings.TrimPrefix(a, "--root="), 1, true
	}
	return "", 0, false
}

const scaffoldBuildGo = `package main

import (
	"fmt"

	"github.com/toyz/gw/gwext"
)

// .gw/build.go — a compiled Go extension for this workspace.
// Full API: https://pkg.go.dev/github.com/toyz/gw/gwext
func main() {
	// A custom command with a typed flag (short form -n):
	//   gw greet   |   gw greet --name gopher   |   gw greet -n gopher
	gwext.Command("greet", "example custom command",
		func(c *gwext.Context) error {
			fmt.Printf("hello %s (%d module(s) at %s)\n",
				c.String("name"), len(c.Modules), c.Root)
			// c.Mod(path) is a typed handle over a module:
			//   c.Mod("example.com/api").Build()    // go build ./...
			//   c.Mod("web").Tool("npm").Run("ci")  // any toolchain
			return nil
		},
		gwext.Str("name", "world", "who to greet").Alias("n"))

	// A lifecycle hook: runs after ` + "`gw sync`" + `.
	gwext.Hook("post-sync", func(c *gwext.Context) error {
		fmt.Println("post-sync: go.work updated")
		return nil
	})

	gwext.Main()
}
`
