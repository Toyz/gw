package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// execFlags are shared by run and every go-passthrough command.
type execFlags struct {
	parallel        bool
	continueOnError bool
	envFiles        []string
	envVars         []string
}

func (f *execFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.parallel, "parallel", "p", false, "run modules concurrently")
	cmd.Flags().BoolVar(&f.continueOnError, "continue-on-error", false, "keep going after a module fails (serial)")
	cmd.Flags().StringArrayVar(&f.envFiles, "env-file", nil, "load environment from a dotenv file (repeatable)")
	cmd.Flags().StringArrayVar(&f.envVars, "env", nil, "set an environment variable KEY=VALUE (repeatable)")
}

// runArgvAcross discovers modules and runs argv in each, printing a summary and
// returning an error that yields the worst module exit code.
func runArgvAcross(cmd *cobra.Command, f execFlags, argv []string) error {
	root, cfg, mods, err := loadWorkspace()
	if err != nil {
		return err
	}
	if len(mods) == 0 {
		return fmt.Errorf("no modules found")
	}
	env, err := workspace.ResolveEnv(root, cfg, f.envFiles, f.envVars)
	if err != nil {
		return err
	}
	fireHook(cmd, root, mods, "pre-"+cmd.Name())
	results := workspace.RunAcross(context.Background(), mods, argv, workspace.ExecOpts{
		Parallel:        f.parallel,
		ContinueOnError: f.continueOnError,
		Env:             env,
		Stdout:          cmd.OutOrStdout(),
		Stderr:          cmd.ErrOrStderr(),
	})
	workspace.PrintSummary(cmd.OutOrStdout(), results)
	fireHook(cmd, root, mods, "post-"+cmd.Name())
	if code := workspace.WorstExit(results); code != 0 {
		os.Exit(code)
	}
	return nil
}

func newRunCmd() *cobra.Command {
	var f execFlags
	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a command in every module's directory",
		Long:  "run executes an arbitrary command inside each module. Put the command after --.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArgvAcross(cmd, f, args)
		},
	}
	f.bind(cmd)
	return cmd
}

// goCmd is a `go` subcommand fanned out across every module. The builtins
// build/test/vet/generate/tidy are all instances of this one shape.
type goCmd struct {
	use     string   // cobra Use string, e.g. "build [packages/flags...]"
	short   string   // one-line help
	base    []string // argv prefix, e.g. {"go", "build"}
	defArgs []string // appended when the user passes no args (nil = pass none)
	noArgs  bool     // reject positional args (tidy)
}

func (gc goCmd) command() *cobra.Command {
	var f execFlags
	accept := cobra.ArbitraryArgs
	if gc.noArgs {
		accept = cobra.NoArgs
	}
	cmd := &cobra.Command{
		Use:   gc.use,
		Short: gc.short,
		Args:  accept,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = gc.defArgs
			}
			argv := append(append([]string{}, gc.base...), args...)
			return runArgvAcross(cmd, f, argv)
		},
	}
	f.bind(cmd)
	return cmd
}

// goCommands are the built-in `go` passthroughs. Add a row to add a command;
// each gets -p/--continue-on-error/--env* and pre-/post-<name> hooks for free.
var goCommands = []goCmd{
	{use: "build [packages/flags...]", short: "Run `go build` in every module (default ./...)", base: []string{"go", "build"}, defArgs: []string{"./..."}},
	{use: "test [packages/flags...]", short: "Run `go test` in every module (default ./...)", base: []string{"go", "test"}, defArgs: []string{"./..."}},
	{use: "vet [packages/flags...]", short: "Run `go vet` in every module (default ./...)", base: []string{"go", "vet"}, defArgs: []string{"./..."}},
	{use: "generate [packages/flags...]", short: "Run `go generate` in every module (default ./...)", base: []string{"go", "generate"}, defArgs: []string{"./..."}},
	{use: "tidy", short: "Run `go mod tidy` in every module", base: []string{"go", "mod", "tidy"}, noArgs: true},
}
