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
| `gw run -- <cmd>` | Run a command in every module's directory. `-p` parallel, `--continue-on-error`. |
| `gw test [args]` | `go test` across every module (default `./...`). `-p` parallel. |
| `gw tidy` | `go mod tidy` across every module. `-p` parallel. |
| `gw list` | List modules; `-v` adds go version + external requires; `--json`. |
| `gw add <path>` / `gw remove <path>` | Add/remove a single module's `use` directive. |
| `gw graph` | Print the intra-workspace dependency DAG (edge A->B = A requires B). Text, `--dot` (Graphviz), or `--json`. Edges come from direct/indirect requires and local `replace` targets. |
| `gw affected --since <ref>` | Diff the working tree against a git ref, map changed files to owning modules, and walk the DAG to every impacted module. `--seeds` (only directly-changed), `--dir`, `--json`. Feed selective CI: `gw affected --since main`. |
| `gw doctor` | One-shot health check: missing/stale `go.work`, use entries with no `go.mod`, modules missing from `go.work`, un-hoisted `replace` directives, and version/directive drift. Exits non-zero on any error (`--strict` also fails on warnings). |

`-C, --root <dir>` sets the workspace root (default: nearest ancestor with a
`go.work`, else the current directory).

## Config (optional `gw.yaml`)

Zero-config works. To customize, drop a `gw.yaml` at the workspace root:

```yaml
root: .
ignore:
  - "examples/**"
  - "**/testdata"
pins:                        # force these versions in `gw lint --fix`
  github.com/foo/bar: v1.4.0
```

Directories `.git`, `vendor`, `testdata`, `node_modules`, `.idea`, `.vscode`
are always skipped.

## CI (GitHub Action)

`gw` ships a composite action. In a repo that uses a workspace:

```yaml
- uses: actions/setup-go@v5
  with: { go-version: stable }
- uses: toyz/gw@v1
  with:
    command: doctor --strict   # default; or "sync --check", "lint", "affected --since main"
```

Inputs: `command` (default `doctor --strict`), `version` (default `latest`),
`working-directory` (default `.`). Requires Go on the runner (`actions/setup-go`).
See [.github/workflows/example-consumer.yml](.github/workflows/example-consumer.yml).

## License

MIT. See [LICENSE](LICENSE).
