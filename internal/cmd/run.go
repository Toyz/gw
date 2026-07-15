package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/internal/workspace"
)

// execFlags are shared by run/test/tidy.
type execFlags struct {
	parallel        bool
	continueOnError bool
}

func (f *execFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.parallel, "parallel", "p", false, "run modules concurrently")
	cmd.Flags().BoolVar(&f.continueOnError, "continue-on-error", false, "keep going after a module fails (serial)")
}

// runArgvAcross discovers modules and runs argv in each, printing a summary and
// returning an error that yields the worst module exit code.
func runArgvAcross(cmd *cobra.Command, f execFlags, argv []string) error {
	_, _, mods, err := loadWorkspace()
	if err != nil {
		return err
	}
	if len(mods) == 0 {
		return fmt.Errorf("no modules found")
	}
	results := workspace.RunAcross(context.Background(), mods, argv, workspace.ExecOpts{
		Parallel:        f.parallel,
		ContinueOnError: f.continueOnError,
		Stdout:          cmd.OutOrStdout(),
		Stderr:          cmd.ErrOrStderr(),
	})
	workspace.PrintSummary(cmd.OutOrStdout(), results)
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

func newTestCmd() *cobra.Command {
	var f execFlags
	cmd := &cobra.Command{
		Use:   "test [packages/flags...]",
		Short: "Run `go test` in every module (default ./...)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"./..."}
			}
			return runArgvAcross(cmd, f, append([]string{"go", "test"}, args...))
		},
	}
	f.bind(cmd)
	return cmd
}

func newTidyCmd() *cobra.Command {
	var f execFlags
	cmd := &cobra.Command{
		Use:   "tidy",
		Short: "Run `go mod tidy` in every module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runArgvAcross(cmd, f, []string{"go", "mod", "tidy"})
		},
	}
	f.bind(cmd)
	return cmd
}
