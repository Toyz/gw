package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toyz/gw/gwext"
	"github.com/toyz/gw/internal/ext"
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

// goInject selects which build-provider outputs a command accepts.
type goInject struct {
	tags    bool // append -tags from provider tags (build/test/vet)
	ldflags bool // append -ldflags "-X k=v" from provider vars (build/test)
}

// runArgvAcross runs prefix+injected-flags+userArgs in every module, printing a
// summary and exiting with the worst module exit code. inj controls whether
// extension build-provider tags/ldflags are woven in between prefix and userArgs.
func runArgvAcross(cmd *cobra.Command, f execFlags, prefix, userArgs []string, inj goInject) error {
	root, cfg, mods, err := loadWorkspace()
	if err != nil {
		return err
	}
	if len(mods) == 0 {
		return fmt.Errorf("no modules found")
	}

	base, info, err := workspaceEnv(root, cfg, mods)
	if err != nil {
		return err
	}
	cli, err := workspace.ResolveCLIEnv(root, f.envFiles, f.envVars)
	if err != nil {
		return err
	}
	env := append(base, cli...)

	argv := append([]string{}, prefix...)
	if inj.tags {
		if tags := dedupStrings(info.Tags); len(tags) > 0 {
			argv = append(argv, "-tags="+strings.Join(tags, ","))
		}
	}
	if inj.ldflags && len(info.Vars) > 0 {
		argv = append(argv, "-ldflags="+ldflagsX(info.Vars))
	}
	argv = append(argv, userArgs...)

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

// workspaceEnv is the env applied to every command gw spawns: config env plus
// extension build-provider env, as sorted KEY=VALUE overrides. It also returns
// the resolved BuildInfo so callers can reuse its vars/tags. CLI --env layers on
// top of this (see runArgvAcross).
func workspaceEnv(root string, cfg workspace.Config, mods []workspace.Module) ([]string, gwext.BuildInfo, error) {
	info, err := ext.Provide(root, toGwextModules(mods))
	if err != nil {
		return nil, info, err
	}
	cfgEnv, err := workspace.ResolveConfigEnv(root, cfg)
	if err != nil {
		return nil, info, err
	}
	return append(cfgEnv, sortedEnv(info.Env)...), info, nil
}

// ldflagsX renders provider vars as a `-ldflags` value: -X key=value pairs,
// sorted, space-joined (go splits the value on spaces).
func ldflagsX(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, "-X "+k+"="+vars[k])
	}
	return strings.Join(parts, " ")
}

// sortedEnv renders a map as sorted KEY=VALUE entries (nil for an empty map).
func sortedEnv(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

// dedupStrings returns the sorted, de-duplicated, non-empty members of in.
func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func newRunCmd() *cobra.Command {
	var f execFlags
	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a command in every module's directory",
		Long:  "run executes an arbitrary command inside each module. Put the command after --.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArgvAcross(cmd, f, args, nil, goInject{})
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
	inject  goInject // which build-provider flags this command accepts
}

func (gc goCmd) command() *cobra.Command {
	var help execFlags // bound only so gw's flags render in --help
	cmd := &cobra.Command{
		Use:   gc.use,
		Short: gc.short,
		// Flag parsing is disabled so go's own flags (-v, -run, -race, -count,
		// ...) pass straight through; splitExecArgs pulls out gw's flags. A bare
		// `--` also forces everything after it to reach `go` verbatim.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if wantsHelp(args) {
				return cmd.Help()
			}
			f, rest := splitExecArgs(args)
			if gc.noArgs && len(rest) > 0 {
				return fmt.Errorf("%q takes no arguments, got %v", cmd.Name(), rest)
			}
			if len(rest) == 0 {
				rest = gc.defArgs
			}
			return runArgvAcross(cmd, f, gc.base, rest, gc.inject)
		},
	}
	help.bind(cmd)
	return cmd
}

// splitExecArgs separates gw's own flags from everything else (go flags and
// package patterns), which pass through to the underlying command untouched.
// A `--` stops gw-flag parsing: the remainder is passed through verbatim.
func splitExecArgs(args []string) (execFlags, []string) {
	var f execFlags
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return f, append(rest, args[i+1:]...)
		// Persistent -C/--root is not parsed by cobra when flag parsing is
		// disabled, so consume it here into the global rootFlag (resolveRoot
		// reads it) instead of leaking it into the go command.
		case a == "-C" || a == "--root":
			if i+1 < len(args) {
				i++
				rootFlag = args[i]
			}
		case strings.HasPrefix(a, "--root="):
			rootFlag = strings.TrimPrefix(a, "--root=")
		case strings.HasPrefix(a, "-C="):
			rootFlag = strings.TrimPrefix(a, "-C=")
		case a == "-p" || a == "--parallel":
			f.parallel = true
		case a == "--continue-on-error":
			f.continueOnError = true
		case a == "--env-file":
			if i+1 < len(args) {
				i++
				f.envFiles = append(f.envFiles, args[i])
			}
		case strings.HasPrefix(a, "--env-file="):
			f.envFiles = append(f.envFiles, strings.TrimPrefix(a, "--env-file="))
		case a == "--env":
			if i+1 < len(args) {
				i++
				f.envVars = append(f.envVars, args[i])
			}
		case strings.HasPrefix(a, "--env="):
			f.envVars = append(f.envVars, strings.TrimPrefix(a, "--env="))
		default:
			rest = append(rest, a)
		}
	}
	return f, rest
}

// wantsHelp reports whether -h/--help appears before any `--` separator.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// goCommands are the built-in `go` passthroughs. Add a row to add a command;
// each gets -p/--continue-on-error/--env* and pre-/post-<name> hooks for free.
// inject marks which commands weave in an extension's build-provider tags/vars.
var goCommands = []goCmd{
	{use: "build [packages/flags...]", short: "Run `go build` in every module (default ./...)", base: []string{"go", "build"}, defArgs: []string{"./..."}, inject: goInject{tags: true, ldflags: true}},
	{use: "test [packages/flags...]", short: "Run `go test` in every module (default ./...)", base: []string{"go", "test"}, defArgs: []string{"./..."}, inject: goInject{tags: true, ldflags: true}},
	{use: "vet [packages/flags...]", short: "Run `go vet` in every module (default ./...)", base: []string{"go", "vet"}, defArgs: []string{"./..."}, inject: goInject{tags: true}},
	{use: "generate [packages/flags...]", short: "Run `go generate` in every module (default ./...)", base: []string{"go", "generate"}, defArgs: []string{"./..."}},
	{use: "tidy", short: "Run `go mod tidy` in every module", base: []string{"go", "mod", "tidy"}, noArgs: true},
}
