# gw

`gw` makes Go workspaces (`go.work`) usable at scale. It auto-generates and
maintains `go.work`, lints cross-module dependency versions, and runs commands
across every module — the way `dotnet sln` and Cargo workspaces already work.

## Install

```
go install github.com/toyz/gw@latest
```

## Commands

| Command | What it does |
| --- | --- |
| `gw init` | Bootstrap an existing multi-module repo: create `go.work` and **move** every `replace` directive out of each `go.mod` up into `go.work`. Refuses to clobber an existing `go.work` (use `--force`). `--dry-run` previews. |
| `gw sync` | Regenerate `go.work`'s `use` set from discovered modules (preserving `replace`/`godebug`), then run `go work sync`. `--check` (CI: exit non-zero if stale), `--dry-run`, `--no-work-sync`. |
| `gw lint` | Report dependencies required at different versions across modules, plus mismatched `go`/`toolchain` directives. Exits non-zero on mismatch. `--fix` aligns dependency versions (`--strategy highest\|lowest`); directive mismatches are reported, never auto-changed. |
| `gw run -- <cmd>` | Run a command in every module's directory. `-p` parallel, `--continue-on-error`. Inject env with `--env-file <f>` (dotenv, repeatable) and `--env KEY=VAL` (repeatable); see [Config](#config-optional-gwtoml). |
| `gw build [args]` | `go build` across every module (default `./...`) — a per-module compile check with a pass/fail summary. `-p` parallel; same `--env-file`/`--env` as `run`. |
| `gw test [args]` | `go test` across every module (default `./...`). `-p` parallel; same `--env-file`/`--env` as `run`. |
| `gw vet [args]` | `go vet` across every module (default `./...`). `-p` parallel; same flags as `run`. |
| `gw generate [args]` | `go generate` across every module (default `./...`). `-p` parallel; same flags as `run`. |
| `gw tidy` | `go mod tidy` across every module. `-p` parallel; same `--env-file`/`--env` as `run`. |
| `gw list` | List modules; `-v` adds go version + external requires; `--json`. |
| `gw add <path>` / `gw remove <path>` | Add/remove a single module's `use` directive. |
| `gw graph` | Print the intra-workspace dependency DAG (edge A->B = A requires B). Text, `--dot` (Graphviz), or `--json`. Edges come from direct/indirect requires and local `replace` targets. |
| `gw affected --since <ref>` | Diff the working tree against a git ref, map changed files to owning modules, and walk the DAG to every impacted module. `--seeds` (only directly-changed), `--dir`, `--json`. Feed selective CI: `gw affected --since main`. |
| `gw doctor` | One-shot health check: missing/stale `go.work`, use entries with no `go.mod`, modules missing from `go.work`, un-hoisted `replace` directives, and version/directive drift. Exits non-zero on any error (`--strict` also fails on warnings). |

`-C, --root <dir>` sets the workspace root (default: nearest ancestor with a
`go.work`, else the current directory).

## Config (optional `gw.toml`)

Zero-config works. To customize, drop a `gw.toml` at the workspace root (TOML is
preferred; a `gw.yaml` / `gw.yml` with the same keys still works — if both exist,
`gw.toml` wins):

```toml
root = "."
ignore = ["examples/**", "**/testdata"]
env_files = ["ci.env"]          # dotenv files layered on top of [env], in order

[pins]                          # force these versions in `gw lint --fix`
"github.com/foo/bar" = "v1.4.0"

[env]                           # applied to run/test/tidy, hooks, and extensions
CGO_ENABLED = "0"
```

Directories `.git`, `.gw`, `vendor`, `testdata`, `node_modules`, `.idea`,
`.vscode` are always skipped.

### Environment injection (opt-in)

Nothing is injected unless you ask for it. Sources, lowest precedence to highest:

1. `[env]` in the config file
2. `env_files` in the config file (in order)
3. `--env-file <path>` on `run`/`test`/`tidy` (repeatable, in order)
4. `--env KEY=VAL` on `run`/`test`/`tidy` (repeatable)

Each layer overlays the ambient process environment; a later layer wins on a key
collision. The config layers (`[env]` + `env_files`) apply to everything gw
spawns — module commands, lifecycle hooks, and extension commands — so they are
the place for workspace-wide settings. The `--env*` flags are per-invocation and
scoped to that command's module runs. Dotenv files support `# comments`, a
leading `export `, and single/double quotes (`\n \t \r \" \\` inside double
quotes); values are otherwise literal (no `$VAR` interpolation).

## CI (GitHub Action)

`gw` ships a composite action. In a repo that uses a workspace:

```yaml
- uses: actions/setup-go@v5
  with: { go-version: stable }
- uses: toyz/gw@v0
  with:
    command: doctor --strict   # default; or "sync --check", "lint", "affected --since main"
```

Inputs: `command` (default `doctor --strict`), `version` (default `latest`),
`working-directory` (default `.`). Requires Go on the runner (`actions/setup-go`).
See [.github/workflows/example-consumer.yml](.github/workflows/example-consumer.yml).

## Extensions (`.gw/build.go`)

Extend gw with your own commands and lifecycle hooks by writing a compiled Go
extension — think Cargo's `build.rs`. Scaffold it:

```
gw ext init      # creates .gw/build.go + .gw/go.mod
gw ext list      # show the extension's commands + hooks
gw ext build     # compile (cached by content hash; auto-built on use)
```

`.gw/build.go` registers commands and hooks against the `gwext` SDK. Because it
is real compiled Go, a command can orchestrate anything — build and run services
in a fixed order, fan out in parallel, or drive polyglot tools:

```go
package main

import (
	"fmt"

	"github.com/toyz/gw/gwext"
)

func main() {
	// `gw boot`: build then run three services in a fixed order.
	gwext.Command("boot", "build+run services in order", func(c *gwext.Context) error {
		for _, p := range []string{"example.com/worker", "example.com/api", "example.com/gateway"} {
			m := c.Mod(p)
			if err := m.Build(); err != nil { // typed: go build ./...
				return err
			}
			if err := m.Run(); err != nil {   // typed: go run .
				return err
			}
		}
		return nil
	})

	// Polyglot: run any tool in a module's directory.
	gwext.Command("web", "start the frontend", func(c *gwext.Context) error {
		return c.Mod("example.com/web").Exec("npm", "run", "dev")
	})

	// Hook: runs automatically after `gw sync`.
	gwext.Hook("post-sync", func(c *gwext.Context) error {
		fmt.Printf("synced %d modules\n", len(c.Modules))
		return nil
	})

	gwext.Main()
}
```

**Context helpers** (on `*gwext.Context`):

- `c.Modules`, `c.Module(path)`, `c.Root`, `c.Args` (command args), `c.Event` (hook).
- `c.Mod(path)` -> typed handle: `.Build() .Test() .Run() .Vet() .Generate() .Tidy()`
  (each defaults to `./...`/`.`), `.Go(args...)`, `.Exec(bin, args...)`, `.Start(...)`
  for long-lived processes.
- Other toolchains: `.Tool(bin)` binds any executable and runs it in the module —
  `c.Mod("web").Tool("yarn").Run("dev")`; `.Start(...)` for long-lived ones.
- `c.Go(dir, args...)`, `c.Run(dir, bin, args...)`, `c.Start(...)` for root-level or
  arbitrary directories.

**Typed command flags:** declare flags with `gwext.Str/Bool/Int` after the
handler. gw parses them from the user's args (the handler reads typed values via
`c.String/c.Bool/c.Int`, with leftover positionals in `c.Args`), and they show up
in `gw <cmd> --help` and `gw ext list`:

```go
gwext.Command("hello", "greet someone", func(c *gwext.Context) error {
	fmt.Println("hello", c.String("name"))
	return nil
}, gwext.Str("name", "world", "who to greet"), gwext.Bool("loud", "shout"))
```

**Build providers** (`gwext.Provide`) let an extension *compute* environment and
Go build settings at run time — gw's analogue of a Cargo build script emitting
`cargo::rustc-env` / `rustc-cfg`. Prebuilt providers ship in `gwext`, so common
needs are one line — e.g. stamp the git SHA into your binary for free:

```go
gwext.Provide(gwext.GitStamp("example.com/app/version")) // -ldflags -X the commit/branch/tag/time
gwext.Provide(gwext.GitEnv())                            // export GW_GIT_COMMIT, GW_GIT_BRANCH, ...
```

`GitStamp` fills string vars in the named package (`Commit`, `Short`, `Branch`,
`Tag`, `Time`, `Dirty`); because `-X` only affects the module that links that
package, the stamp lands there alone. Roll your own for anything else:

```go
gwext.Provide(func(c *gwext.Context) (gwext.BuildInfo, error) {
	return gwext.BuildInfo{
		Env:  map[string]string{"PORT": derivePort(c)},          // process env for every command
		Vars: map[string]string{"example.com/app/build.User": me}, // -ldflags -X (build/test)
		Tags: []string{"production"},                            // build tags (build/test/vet)
	}, nil
})
```

`Env` layers into every command between config and `--env` (so `--env` still
wins). `Vars` become `-ldflags "-X key=value"` and `Tags` become `-tags` on the
compiling commands. Providers may print freely — that goes to stderr, never the
result. `gwext.Git(dir)` returns the raw `GitInfo` if you want to build your own.

**Overriding builtins:** a plain `gwext.Command` whose name collides with a
builtin is ignored (builtins win). To *replace* one — e.g. wrap `gw test` with
setup/teardown — register it with `gwext.Override("test", ...)`, and gw removes
the shadowed builtin. `gwext.Hide("tidy", "generate")` drops builtins entirely.
`gw ext list` shows which commands override and which builtins are hidden.

**Hook events:** `pre-`/`post-` for `sync`, `lint`, `run`, `build`, `test`, `vet`,
`generate`, `tidy` (e.g. `post-sync`, `pre-test`). The compiled binary is cached under `.gw/bin/`
(git-ignored) and rebuilt only when `.gw` sources change.

## License

MIT. See [LICENSE](LICENSE).
