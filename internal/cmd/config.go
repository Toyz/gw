package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Scaffold and locate the workspace's gw.toml",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigPathCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write a commented starter gw.toml in the workspace root",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if existing, ok := workspace.ConfigPath(root); ok {
				return failf("%s already exists", filepath.Base(existing)).
					withHint("edit it directly, or remove it to re-scaffold")
			}
			dst := filepath.Join(root, workspace.DefaultConfigName)
			if err := os.WriteFile(dst, []byte(scaffoldConfig), 0o644); err != nil {
				return err
			}
			p.ok("wrote %s", workspace.DefaultConfigName)
			p.hint("every field is commented out — uncomment what you need")
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file gw loads for this workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			path, ok := workspace.ConfigPath(root)
			if !ok {
				return failf("no gw config in %s", root).
					withHint("run `gw config init` to create one")
			}
			p.println(path)
			return nil
		},
	}
}

// scaffoldConfig is the commented starter gw.toml written by `gw config init`.
// Top-level keys sit above the [tables] so uncommenting env_files can't land it
// inside [env]; the defaults comment mirrors workspace.defaultIgnores.
const scaffoldConfig = `# gw.toml — workspace configuration. Every field is optional; uncomment what
# you need. Reference: https://github.com/toyz/gw

# Relocate the workspace root (relative to this file, or absolute).
# root = "services"

# Extra path globs to skip during module discovery, on top of the built-in
# ignores (.git .gw vendor testdata node_modules .idea .vscode).
# ignore = ["examples/*", "**/testdata"]

# Dotenv files layered over [env], in order. Keep top-level keys above any
# [table] so TOML doesn't parse them into it.
# env_files = [".env", ".env.local"]

# Pin dependencies to exact versions; steers gw lint --fix.
# [pins]
# "github.com/aws/aws-sdk-go-v2" = "v1.30.0"

# Environment applied to every command, hook, and extension gw runs.
# [env]
# CGO_ENABLED = "0"

# Custom commands and lifecycle hooks — run natively, no .gw/build.go. A step
# "<module>:<verb>" (build/test/vet/generate/tidy/run) runs that go command in
# the module; any other string is a shell command run in dir (else the root).
# [commands.boot]
# desc = "build services, then codegen"
# steps = ["worker:build", "api:build", "sqlc generate"]

# [hooks.pre-build]
# steps = ["sqlc generate"]
`
