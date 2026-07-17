# gw

`gw` is the workflow tool for **Go monorepos**. It turns a multi-module repo —
many `go.mod` under one roof — into a coherent workspace: it auto-generates and
maintains `go.work`, keeps one dependency version across modules, runs the go
tool everywhere, and tells you exactly what a change affects. What Nx and
Turborepo are for JavaScript, and Cargo workspaces are for Rust — for Go.

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
| `gw affected --since <ref>` | Diff the working tree against a git ref, map changed files to owning modules, and walk the DAG to every impacted module. `--seeds` (only directly-changed), `--dir`, `--json`. `--services` instead lists the `[services.<name>]` a diff touches (by directory) — change-based redeploy across languages, even non-Go. Feed selective CI: `gw affected --since main`. |
| `gw doctor` | One-shot health check: missing/stale `go.work`, use entries with no `go.mod`, modules missing from `go.work`, un-hoisted `replace` directives, and version/directive drift. Exits non-zero on any error (`--strict` also fails on warnings). |
| `gw config init` | Scaffold a commented starter `gw.toml` in the workspace root (won't clobber an existing config). `gw config path` prints the config file gw loads. See [Config](#config-optional-gwtoml). |
| `gw verify` | Check the **release contract** workspace mode hides: every require on another workspace module must resolve to a **real published tag** whose code still matches what's on disk. Inside the workspace such a require resolves to local code, so `go build` passes even when the version was never tagged — `verify` runs the checks an external consumer (or `GOWORK=off` release build) would hit. Also flags local-path `replace` leaks, and prints a release plan in dependency order. Exits non-zero on errors (`--strict` also on warnings); `--json`. |
| `gw mcp` | Run gw as a **Model Context Protocol** stdio server so an AI agent can drive the workspace. See [MCP](#mcp-server). |

`-C, --root <dir>` sets the workspace root (default: nearest ancestor with a
`go.work`, else the current directory).

## Config (optional `gw.toml`)

Zero-config works. To customize, run `gw config init` for a commented starter
`gw.toml` in the workspace root (`gw config path` prints the file gw loads). TOML
is preferred; a `gw.yaml` / `gw.yml` with the same keys still works — if both
exist, `gw.toml` wins:

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

### Commands & hooks (no `.gw/build.go`)

Declare custom commands and lifecycle hooks right in the config — run natively by
gw, no compiled extension needed:

```toml
[commands.boot]                 # adds `gw boot`
desc = "build services, then codegen"
steps = [
  "worker:build",               # <module>:<verb> → go build ./... in the module
  "api:build",
  "sqlc generate",              # any other string → shell command
]
dir = "services"                # working dir for shell steps

[hooks.pre-build]               # runs before `gw build`
steps = ["sqlc generate"]
```

Each step is either `<module>:<verb>` (verb ∈
`build test vet generate tidy run`, run as that go command in the module's
directory) or a shell command (run in `dir`, else the root). Hooks are keyed by
event — `pre-`/`post-<command>` — the same events the [gwext SDK](#extensions-gwbuildgo)
uses. A compiled extension wins any name/event collision; config fills the rest.
For real logic (loops, conditionals), use a `.gw/build.go` extension.

### Services (polyglot `affected`)

A Go module is a *build* unit; a **service** is a *deployable* unit — and it need
not be Go. Declare deployable dirs so `gw affected --since <ref> --services` reports
which ones a diff touches (by directory), even non-Go ones the Go workspace can't
see. Change-based redeploy across languages:

```toml
[services.api]
path = "svc/api"                # dir (relative to root); defaults to the name

[services.sat]                  # a Rust service — no go.mod, still tracked
path = "sat"
lang = "rust"                   # informational
build = "cargo build --release" # informational (used by deploy tooling, not gw core)
port = 8080
```

gw core only uses `path` (the rest is metadata for a deploy step / boot extension).
`gw affected --since main --services` prints the affected service names, one per
line — pipe it straight into your deploy. In `--json`, affected services appear
under `services` alongside `seeds`/`impacted`.

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

Full API reference: **[pkg.go.dev/github.com/toyz/gw/gwext](https://pkg.go.dev/github.com/toyz/gw/gwext)**

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

	// Hook: runs automatically after `gw sync`. Before/After wrap any command —
	// every built-in (typed constants: gwext.Sync, gwext.Build, …) and your own
	// custom verbs (a plain string, e.g. Before("boot", …)).
	gwext.After(gwext.Sync, func(c *gwext.Context) error {
		fmt.Printf("synced %d modules\n", len(c.Modules))
		return nil
	})

	gwext.Main()
}
```

Hooks run for every workspace command, `pre-<cmd>` before and `post-<cmd>`
after (post- on success; `--dry-run`/`--check` runs skip them). The older
`gwext.Hook("post-sync", …)` string form still works but is deprecated.

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
in `gw <cmd> --help` and `gw ext list`. Both `--flag value` and `--flag=value`
forms parse; add short aliases with `.Alias(...)` (either name sets the same
value, read by the canonical name). `gwext.Strs(name, help)` declares a
repeatable slice flag — pass it multiple times and/or comma-separated
(`--tag a --tag b`, `--tag a,b`) and read with `c.Strings(name)`:

```go
gwext.Command("hello", "greet someone", func(c *gwext.Context) error {
	fmt.Println("hello", c.String("name"))
	return nil
}, gwext.Str("name", "world", "who to greet").Alias("n"), gwext.Bool("loud", "shout").Alias("l"))
```

**Build providers** (`gwext.Provide`) let an extension *compute* environment and
Go build settings at run time — gw's analogue of a Cargo build script emitting
`cargo::rustc-env` / `rustc-cfg`. Prebuilt providers ship in `gwext`, so common
needs are one line — e.g. stamp the git SHA into your binary for free:

```go
gwext.Provide(gwext.GitStamp("example.com/app/version")) // -ldflags -X the commit/branch/tag/time
gwext.Provide(gwext.GitEnv())                            // export GW_GIT_COMMIT, GW_GIT_BRANCH, ...

// In CI, prefer the runner's env — reliable on shallow / detached checkouts:
gwext.Provide(gwext.CIStamp("example.com/app/version")) // GitHub Actions + GitLab CI
gwext.Provide(gwext.CIEnv())                            // export GW_CI_COMMIT, GW_CI_TAG, ...
```

`GitStamp` fills string vars in the named package (`Commit`, `Short`, `Branch`,
`Tag`, `Time`, `Dirty`); `CIStamp` fills the CI-sourced set (`Provider`,
`Commit`, `Ref`, `Tag`, `RunID`, `Repo`, `Actor`). Because `-X` only affects the
module that links the package, the stamp lands there alone. Git/CI built-ins are
workspace-global — pair them with `Provide`, not `ProvideEach`. Roll your own for
anything else:

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
result. `gwext.Git(dir)` and `gwext.CI()` return the raw `GitInfo` / `CIInfo` for
building your own.

**Overriding builtins — decorate, don't shadow.** A plain `gwext.Command` whose
name collides with a builtin is ignored (builtins win). To extend one, register
`gwext.Override("run", ...)`. The right pattern is a *superset*: handle your added
flag, then call through to the original with `c.Builtin` so the vanilla
invocation behaves exactly as before. Never make the override a silent swap that
breaks the original — extend the verb, don't replace its meaning.

```go
gwext.Override("run", "run across modules; adds --mode=all launcher",
	func(c *gwext.Context) error {
		if c.Bool("mode-all") {
			return launchEverything(c) // the new behavior
		}
		return c.Builtin("run", c.Args...) // fall through to the real builtin
	},
	gwext.Bool("mode-all", "launch all services (orchestrated)"),
)
```

`c.Builtin(name, args...)` runs gw's own builtin in a child process with
extensions disabled, so the override never recurses into itself. Overrides are
always **surfaced, never silent**: `gw --help` labels the verb `(overrides
builtin)` and `gw ext list` marks it `override`, so anyone can see a builtin is
extended here. `gwext.Hide("tidy", "generate")` drops builtins entirely (also
listed by `gw ext list`).

Because the call-through child runs with extensions off, a `c.Builtin`
fall-through executes the *pure* builtin: lifecycle **hooks are suppressed**
during it (this is what prevents an override from recursing) — build/test
providers and env injection still apply. So `gw build` invoked directly fires
`pre-/post-build` hooks, while a `build` override that calls `c.Builtin("build")`
does not; run any such logic in the override itself. Works for every builtin —
flag-parsed (`run`), go-passthrough (`build`/`test`/`vet`/`generate`/`tidy`),
and the rest (`sync`/`lint`/`doctor`/`graph`/`list`/...).

**Hook events:** `pre-`/`post-` for `sync`, `lint`, `run`, `build`, `test`, `vet`,
`generate`, `tidy` (e.g. `post-sync`, `pre-test`). The compiled binary is cached under `.gw/bin/`
(git-ignored) and rebuilt only when `.gw` sources change.

## MCP server

`gw mcp` runs gw as a [Model Context Protocol](https://modelcontextprotocol.io)
stdio server, so an agent can inspect and drive a Go workspace directly. Register
it with Claude Code:

```
claude mcp add gw -- gw mcp
```

Tools exposed:

| Tool | What it returns |
| --- | --- |
| `gw_list` | modules, dirs, go versions, direct requires (JSON) |
| `gw_doctor` | health issues + error/warning/info counts |
| `gw_lint` | cross-module dependency + go/toolchain version drift |
| `gw_graph` | the intra-workspace dependency DAG (nodes + edges) |
| `gw_affected` | `{since}` → seeds + transitively impacted modules (selective CI) |
| `gw_sync` | regenerate `go.work` (`{check}` reports drift without writing) |
| `gw_test` | `go test` across every module + pass/fail summary |

Read-only insight tools (`list`/`doctor`/`lint`/`graph`/`affected`) call gw's
workspace engine directly and return structured JSON; action tools (`sync`,
`test`) invoke gw so they inherit hooks and build providers. Every tool takes an
optional `root` (defaults to the nearest `go.work`).

## License

MIT. See [LICENSE](LICENSE).
