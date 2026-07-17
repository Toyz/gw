import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import "../tip"; // registers the `tip` hover-tooltip attribute

const IGNORE_DEFAULTS = [
  ".git", ".gw", "vendor", "testdata", "node_modules", ".idea", ".vscode",
];

// Every field on the gw config, in format-neutral terms (the TOML vs YAML
// spelling difference lives in the "File & formats" section, not here). `tip`
// is a hover example, shown via the <code tip> tooltip attribute.
const FIELDS: { key: string; type: string; desc: string; tip: string }[] = [
  {
    key: "root",
    type: "string",
    desc: "Workspace root — relative to the config file, or absolute. Everything resolves from here.",
    tip: 'root = "services"',
  },
  {
    key: "ignore",
    type: "list",
    desc: "Path globs skipped during module discovery, on top of the built-in ignores.",
    tip: 'ignore = ["examples/*", "**/testdata"]',
  },
  {
    key: "pins",
    type: "map",
    desc: "Pin dependencies to exact versions; steers gw lint --fix. Keyed by module path.",
    tip: '[pins]   "module/path" = "v1.2.3"',
  },
  {
    key: "env",
    type: "map",
    desc: "KEY=VALUE injected into every command, hook, and extension gw spawns.",
    tip: '[env]   CGO_ENABLED = "0"',
  },
  {
    key: "env_files",
    type: "list",
    desc: "Dotenv files layered over env, in order. Later files and CLI --env win.",
    tip: "TOML: env_files   ·   YAML: envFiles",
  },
  {
    key: "commands",
    type: "map",
    desc: "Custom gw <name> verbs run natively — module:verb + shell steps, no build.go.",
    tip: '[commands.boot]   steps = ["api:build"]',
  },
  {
    key: "hooks",
    type: "map",
    desc: "Lifecycle hooks keyed by event (pre-build, post-sync). Same shape as commands.",
    tip: '[hooks.pre-build]   steps = ["sqlc generate"]',
  },
  {
    key: "services",
    type: "map",
    desc: "Deployable units — even non-Go. gw affected --services reports which a diff touches, by directory.",
    tip: '[services.sat]   path = "sat"   lang = "rust"',
  },
];

const FULL_SAMPLE = `# gw.toml — every field is optional; no file = zero config
root = "services"
ignore = ["examples/*", "**/testdata"]
env_files = [".env", ".env.ci"]

[pins]
"github.com/aws/aws-sdk-go-v2" = "v1.30.0"

[env]
CGO_ENABLED = "0"
BUILD_ENV = "ci"`;

const YAML_SAMPLE = `# gw.yaml — same fields, camelCase envFiles
root: services
ignore: ["examples/*", "**/testdata"]
envFiles: [".env", ".env.ci"]
pins:
  golang.org/x/net: v0.27.0
env:
  CGO_ENABLED: "0"
  BUILD_ENV: ci`;

const DISCOVERY_SAMPLE = `# root: relative to gw.toml, or absolute
root = "services"
ignore = ["vendor-*", "**/*_gen", "examples/*"]`;

const PINS_SAMPLE = `[pins]
"github.com/aws/aws-sdk-go-v2" = "v1.30.0"
"golang.org/x/net" = "v0.27.0"`;

const ENV_SAMPLE = `env_files = [".env", ".env.local"]

[env]
CGO_ENABLED = "0"
GOFLAGS = "-mod=mod"`;

const COMMANDS_SAMPLE = `# gw boot — module ops and shell in one ordered list
[commands.boot]
desc = "build services, then codegen"
steps = [
  "worker:build",     # module op → go build ./...
  "api:build",
  "sqlc generate",    # shell command
]

# gw web — a shell tool in a module's directory
[commands.web]
steps = ["npm run dev"]
dir = "web"

# lifecycle hooks, keyed by event — same step list
[hooks.pre-build]
steps = ["sqlc generate"]

[hooks.post-sync]
steps = ["proto:generate"]`;

const SERVICES_SAMPLE = `# deployable units — even non-Go
[services.api]
path = "svc/api"              # dir; defaults to the name

[services.sat]               # a Rust bird, no go.mod
path = "sat"
lang = "rust"                # metadata
build = "cargo build --release"
port = 8080

# gw affected --since main --services  ->  sat`;

const cfgStyles = css`
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

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-top: 1.2rem;
  }
  .chip {
    font-family: var(--mono);
    font-size: 0.74rem;
    color: var(--dim);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.22rem 0.5rem;
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
  .note b {
    color: var(--text);
    font-weight: 600;
  }
  .note code {
    font-family: var(--mono);
    font-size: 0.86em;
    color: var(--teal);
  }

  .tbl-wrap {
    overflow-x: auto;
    border: 1px solid var(--border-soft);
    border-radius: 12px;
    background: var(--panel);
  }
  .tbl {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  .tbl th {
    text-align: left;
    font-family: var(--mono);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.13em;
    color: var(--dim);
    font-weight: 400;
    padding: 0.95rem 1.2rem 0.7rem;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  .tbl td {
    padding: 0.82rem 1.2rem;
    border-bottom: 1px solid var(--border-soft);
    color: #aeb6bf;
    vertical-align: top;
  }
  .tbl tr:last-child td {
    border-bottom: none;
  }
  .tbl .f {
    font-family: var(--mono);
    color: var(--teal);
    white-space: nowrap;
    cursor: help;
    text-decoration: underline dotted var(--border);
    text-underline-offset: 4px;
  }
  .tbl .f small {
    display: block;
    margin-top: 0.25rem;
    color: var(--dim);
    font-size: 0.82em;
  }
  .tbl .t {
    font-family: var(--mono);
    font-size: 0.82rem;
    color: var(--dim);
    white-space: nowrap;
  }
`;

@route("/config")
@component("page-config")
@styles(base, cfgStyles)
export class PageConfig extends LoomElement {
  update() {
    return (
      <div class="wrap">
        <div class="hero">
          <div>
            <div class="kicker mono">
              <b>//</b> gw.toml
            </div>
            <h1>
              Configured in <em>one file</em>.
            </h1>
            <p class="lead">
              gw needs <b>zero</b> configuration. When you want more control,
              drop a <b>gw.toml</b> (or <b>gw.yaml</b>) in the workspace root:
              ignore globs, version pins, environment injection — even custom
              <b> commands</b> and <b>lifecycle hooks</b>, no compiled extension
              required. Every field is optional.
            </p>
          </div>
          <gw-code title="gw.toml" lang="toml" src={FULL_SAMPLE} />
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="terminal" size={14} /> Scaffold
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gw config init</code> writes a commented starter{" "}
                <code>gw.toml</code> in the workspace root — every field present
                but commented, so you uncomment what you need. It won't clobber an
                existing config.
              </p>
              <p>
                <code>gw config path</code> prints which file gw loads
                (<code>gw.toml</code>, <code>gw.yaml</code>, or{" "}
                <code>gw.yml</code>).
              </p>
            </div>
            <gw-term
              title="scaffold"
              lines={[
                { c: "prompt", t: "gw config init" },
                { c: "ok", t: "✓ wrote gw.toml" },
                {
                  c: "dim",
                  t: "help: every field is commented out — uncomment what you need",
                },
              ]}
            />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="sliders" size={14} /> All fields
          </div>
          <div class="tbl-wrap">
            <table class="tbl">
              <thead>
                <tr>
                  <th>Field</th>
                  <th>Type</th>
                  <th>Description</th>
                </tr>
              </thead>
              <tbody>
                {FIELDS.map((f) => (
                  <tr>
                    <td class="f" tip={f.tip}>
                      {f.key}
                    </td>
                    <td class="t">{f.type}</td>
                    <td>{f.desc}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-merge" size={14} /> Commands &amp; hooks
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>[commands.&lt;name&gt;]</code> adds a{" "}
                <code>gw &lt;name&gt;</code> verb;{" "}
                <code>[hooks.&lt;event&gt;]</code> runs on a lifecycle event like{" "}
                <code>pre-build</code> or <code>post-sync</code>. Both run
                natively — <b>no compiled .gw/build.go</b>.
              </p>
              <p>
                Steps run in order. A step like{" "}
                <code tip="→ go build ./... in the api module">"api:build"</code>{" "}
                is a gw module op —{" "}
                <code>&lt;module&gt;:&lt;verb&gt;</code>, run as that go command in
                the module (verbs:{" "}
                <code tip="build · test · vet · generate · tidy · run">
                  build test vet generate tidy run
                </code>
                ). Any other string is a <b>shell command</b> (run in{" "}
                <code>dir</code>, else the root). Mix them freely.
              </p>
              <p>
                Module-relative go tools want the <code>module:verb</code> form —
                a bare <code>go generate ./...</code> shell step runs from the
                root, which has no <code>go.mod</code>, and fails.
              </p>
              <div class="note">
                A compiled extension wins any name/event collision; config fills
                the rest. For real logic — loops, conditionals — reach for a{" "}
                <code>.gw/build.go</code> extension.
              </div>
            </div>
            <gw-code title="gw.toml" lang="toml" src={COMMANDS_SAMPLE} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="package" size={14} /> Services (polyglot affected)
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                A Go module is a <b>build</b> unit; a{" "}
                <code>[services.&lt;name&gt;]</code> is a <b>deployable</b> unit —
                and it need not be Go. Declare deployable dirs and{" "}
                <code>gw affected --since &lt;ref&gt; --services</code> reports
                which ones a diff touches (by directory), even a{" "}
                <b>Rust service the Go workspace can't see</b>.
              </p>
              <p>
                gw core only uses <code>path</code> (defaults to the name); the
                rest — <code>lang</code>, <code>build</code>, <code>port</code>,{" "}
                <code>image</code> — is metadata for a deploy step or a{" "}
                <code>boot</code> extension. In <code>--json</code>, affected
                services appear under <code>services</code> alongside{" "}
                <code>seeds</code>/<code>impacted</code>.
              </p>
              <div class="note">
                Change-based redeploy across languages: pipe{" "}
                <code>gw affected --since main --services</code> straight into
                your deploy — one service name per line.
              </div>
            </div>
            <gw-code title="gw.toml" lang="toml" src={SERVICES_SAMPLE} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="layers" size={14} /> File &amp; formats
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                gw reads the first of <code>gw.toml</code>, <code>gw.yaml</code>,
                or <code>gw.yml</code> found in the workspace root. A missing file
                is not an error — you get zero-config defaults.
              </p>
              <p>
                TOML and YAML carry the same fields; YAML spells one key
                differently (<code>envFiles</code> vs TOML's{" "}
                <code>env_files</code>).
              </p>
            </div>
            <gw-code title="gw.yaml" lang="yaml" src={YAML_SAMPLE} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="folder" size={14} /> Discovery
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>root</code> relocates the workspace root — relative to the
                config file, or absolute. Discovery, <code>go.work</code>, and
                every command resolve from there.
              </p>
              <p>
                <code>ignore</code> is a list of path globs (matched per segment,
                relative to the root) skipped during module discovery. It layers
                on top of the set gw never walks into:
              </p>
              <div class="chips">
                {IGNORE_DEFAULTS.map((d) => (
                  <span class="chip">{d}</span>
                ))}
              </div>
            </div>
            <gw-code title="gw.toml" lang="toml" src={DISCOVERY_SAMPLE} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="lock" size={14} /> Version pins
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>[pins]</code> forces specific dependencies to exact
                versions. When <code>gw lint --fix</code> aligns cross-module
                drift, a pinned module converges on <b>your</b> version instead of
                the highest or lowest found across the workspace.
              </p>
              <p>
                Keyed by module path. Pins only steer <code>--fix</code>;{" "}
                <code>gw lint</code> still reports the drift.
              </p>
            </div>
            <gw-code title="gw.toml" lang="toml" src={PINS_SAMPLE} />
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="key" size={14} /> Environment
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>[env]</code> injects <code>KEY=VALUE</code> into every
                command gw spawns — <code>build</code>/<code>test</code>/
                <code>run</code>/<code>tidy</code>, lifecycle hooks, and extension
                commands. Opt-in: nothing is set unless you declare it.
              </p>
              <p>
                <code>env_files</code> layers dotenv files (relative to the root,
                or absolute) on top, in order — full dotenv syntax:{" "}
                <code>export</code>, <code>#</code> comments, single/double
                quotes, and <code>$VAR</code> / <code>${"{VAR:-default}"}</code>{" "}
                expansion.
              </p>
              <div class="note">
                <b>Precedence</b>, lowest to highest: <code>[env]</code> →{" "}
                <code>env_files</code> → <code>--env-file</code> →{" "}
                <code>--env</code>. Build providers from extensions layer between
                config and the CLI flags.
              </div>
            </div>
            <gw-code title="gw.toml" lang="toml" src={ENV_SAMPLE} />
          </div>
        </section>
      </div>
    );
  }
}
