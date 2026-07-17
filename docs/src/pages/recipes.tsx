import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import "../win"; // <gw-term> / <gw-code>

// ── samples ──

const HERO_TERM = [
  { c: "prompt", t: "gw init" },
  { c: "dim", t: "  create go.work, hoist replaces out of every go.mod" },
  { c: "prompt", t: "gw up" },
  { c: "dim", t: "  your config command: codegen → build → run" },
  { c: "prompt", t: "gw affected --since main" },
  { c: "prompt", t: "gw verify --strict" },
];

const BOOTSTRAP_TERM = [
  { c: "prompt", t: "gw init" },
  { c: "ok", t: "✓ wrote go.work: 7 module(s)" },
  { c: "dim", t: "  hoisted 3 replace directives up into go.work" },
  { c: "prompt", t: "gw doctor" },
  { c: "ok", t: "✓ workspace healthy" },
];

const ORCHESTRATE_TOML = `# gw.toml — bring the stack up, codegen first
[commands.up]
desc = "generate, build, then launch"
steps = [
  "sqlc generate",      # shell — runs at the workspace root
  "proto:generate",     # go generate in the proto module
  "worker:build",
  "api:build",
  "gateway:build",
]

# regenerate before every \`gw build\`, automatically
[hooks.pre-build]
steps = ["sqlc generate"]`;

const STAMP_GO = `// .gw/build.go — version metadata in every binary
package main

import "github.com/toyz/gw/gwext"

func main() {
    // git locally, the CI runner in a pipeline — both stamp the
    // same package; -X only binds in the module that links it.
    gwext.Provide(gwext.GitStamp("example.com/app/version"))
    gwext.Provide(gwext.CIStamp("example.com/app/version"))
    gwext.Main()
}`;

const CI_YAML = `# .github/workflows/ci.yml
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }        # affected needs history
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - uses: toyz/gw@v0
        with: { command: sync --check } # fail if go.work is stale
      - run: gw affected --since origin/\${{ github.base_ref }}`;

const VERIFY_TERM = [
  { c: "prompt", t: "gw verify --strict" },
  { c: "dim", t: "  checking every intra-workspace require resolves to a published tag…" },
  { c: "fail", t: "✗ example.com/gateway requires example.com/api@v0.3.0 — tag not found" },
  { c: "dim", t: "  help: tag example.com/api v0.3.0, or release in dependency order" },
];

const recipeStyles = css`
  .hero {
    display: grid;
    grid-template-columns: minmax(0, 1.02fr) minmax(0, 1.08fr);
    gap: 3.5rem;
    align-items: center;
    padding: 5rem 0 4.5rem;
  }
  @media (max-width: 860px) {
    .hero {
      grid-template-columns: minmax(0, 1fr);
      gap: 2.5rem;
      padding: 3rem 0;
    }
  }
  .kicker {
    font-size: 0.82rem;
    color: var(--dim);
    margin-bottom: 1.1rem;
  }
  .kicker b {
    color: var(--teal);
    font-weight: 400;
  }
  h1 {
    font-size: clamp(2.6rem, 5vw, 3.7rem);
    line-height: 1.02;
    letter-spacing: -0.035em;
    font-weight: 680;
    margin: 0 0 1.2rem;
  }
  h1 em {
    font-style: normal;
    color: var(--amber);
  }
  .lead {
    font-size: 1.1rem;
    color: #aeb6bf;
    max-width: 48ch;
    margin: 0;
  }
  .lead b {
    color: var(--text);
    font-weight: 600;
  }

  .grid2 {
    display: grid;
    grid-template-columns: minmax(0, 0.92fr) minmax(0, 1.08fr);
    gap: 2.6rem;
    align-items: start;
  }
  @media (max-width: 820px) {
    .grid2 {
      grid-template-columns: minmax(0, 1fr);
      gap: 1.6rem;
    }
  }
  .doc p {
    margin: 0 0 1rem;
    color: #aeb6bf;
  }
  .doc p:last-child {
    margin-bottom: 0;
  }
  .doc b {
    color: var(--text);
    font-weight: 600;
  }
  .doc code {
    font-family: var(--mono);
    font-size: 0.88em;
    color: var(--teal);
  }
  .prob {
    display: inline-block;
    font-size: 0.82rem;
    color: var(--amber);
    margin-bottom: 0.75rem;
  }
  .note {
    margin-top: 1.2rem;
    padding: 0.9rem 1.1rem;
    border: 1px solid var(--border-soft);
    border-left: 2px solid var(--amber);
    border-radius: 8px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.012), transparent);
    color: var(--dim);
    font-size: 0.9rem;
  }
  .note code {
    font-family: var(--mono);
    font-size: 0.86em;
    color: var(--teal);
  }
`;

@route("/recipes")
@component("page-recipes")
@styles(base, recipeStyles)
export class PageRecipes extends LoomElement {
  update() {
    return (
      <div class="wrap">
        <div class="hero">
          <div>
            <div class="kicker mono">
              <b>//</b> cookbook
            </div>
            <h1>
              Real workspaces, <em>end to end</em>.
            </h1>
            <p class="lead">
              Five ways teams actually run gw — <b>onboard</b> a monorepo,{" "}
              <b>orchestrate</b> builds, <b>stamp</b> versions, <b>gate</b> CI,
              and <b>guard</b> releases. Copy, adapt, ship.
            </p>
          </div>
          <gw-term title="workspace — gw" lines={HERO_TERM} />
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="folder" size={14} /> Onboard an existing monorepo
          </div>
          <div class="grid2">
            <div class="doc">
              <span class="prob">
                You have modules and per-repo <code>replace</code> directives,
                but no <code>go.work</code>.
              </span>
              <p>
                <code>gw init</code> discovers every module, writes{" "}
                <code>go.work</code>, and <b>hoists</b> each local{" "}
                <code>replace</code> up out of the individual{" "}
                <code>go.mod</code> files into the workspace — so the repo builds
                as one unit. <code>gw doctor</code> confirms it's healthy.
              </p>
              <p>
                From here <code>gw sync</code> keeps the use set current as
                modules come and go.
              </p>
            </div>
            <gw-term title="terminal" lines={BOOTSTRAP_TERM} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-merge" size={14} /> Orchestrate builds &amp;
            codegen
          </div>
          <div class="grid2">
            <div class="doc">
              <span class="prob">
                Bring up a polyglot stack in the right order — with codegen that
                must run first.
              </span>
              <p>
                A <code>[commands.up]</code> in <code>gw.toml</code> runs its
                steps in order: <code>sqlc generate</code> and{" "}
                <code>npm</code> as shell, <code>proto:generate</code> /{" "}
                <code>api:build</code> as module ops in their own directories.
                No compiled extension.
              </p>
              <p>
                A <code>[hooks.pre-build]</code> regenerates before <b>every</b>{" "}
                <code>gw build</code> — codegen can never go stale.
              </p>
            </div>
            <gw-code title="gw.toml" lang="toml" src={ORCHESTRATE_TOML} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="zap" size={14} /> Stamp version info that's always
            right
          </div>
          <div class="grid2">
            <div class="doc">
              <span class="prob">
                You want the commit/tag baked into binaries — and it has to be
                correct in CI too.
              </span>
              <p>
                <code>GitStamp</code> stamps git metadata locally;{" "}
                <code>CIStamp</code> reads the CI runner's env, which stays right
                on the shallow, detached checkouts CI does — where{" "}
                <code>git describe</code> can't. Declare matching{" "}
                <code>var</code>s in the version package; <code>-X</code> only
                binds in the module that links it.
              </p>
              <div class="note">
                Both are workspace-global — pair with <code>Provide</code>, not{" "}
                <code>ProvideEach</code>.
              </div>
            </div>
            <gw-code title=".gw/build.go" lang="go" src={STAMP_GO} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-branch" size={14} /> Test only what a PR touched
          </div>
          <div class="grid2">
            <div class="doc">
              <span class="prob">
                Your monorepo CI rebuilds the world on every PR. Most of it
                didn't change.
              </span>
              <p>
                <code>gw affected --since main</code> diffs the working tree,
                maps changed files to owning modules, and walks the dependency
                DAG to <b>everything impacted</b>. Feed it a job matrix and skip
                the rest.
              </p>
              <p>
                Pair it with <code>gw sync --check</code> to fail fast when{" "}
                <code>go.work</code> drifts. <code>fetch-depth: 0</code> gives{" "}
                <code>affected</code> the history it needs.
              </p>
            </div>
            <gw-code title=".github/workflows/ci.yml" lang="yaml" src={CI_YAML} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="shield-check" size={14} /> Guard the release
            contract
          </div>
          <div class="grid2">
            <div class="doc">
              <span class="prob">
                It builds in the workspace — then breaks for anyone who{" "}
                <code>go get</code>s it.
              </span>
              <p>
                Workspace mode makes an intra-workspace <code>require</code>{" "}
                resolve to <b>local code</b>, so <code>go build</code> passes
                even when that version was never tagged.{" "}
                <code>gw verify --strict</code> runs the checks an external
                consumer (or a <code>GOWORK=off</code> release build) actually
                hits, and prints a release plan in dependency order.
              </p>
              <p>Run it in CI and you never ship an unpublishable tag.</p>
            </div>
            <gw-term title="terminal" lines={VERIFY_TERM} />
          </div>
        </section>
      </div>
    );
  }
}
