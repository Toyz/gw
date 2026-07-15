import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import { yamlLines } from "../highlight";

const MINIMAL = `- uses: actions/setup-go@v5
  with: { go-version: stable }
- uses: toyz/gw@v0
  with:
    command: doctor --strict`;

const AFFECTED = `- uses: actions/checkout@v5
  with: { fetch-depth: 0 }   # affected needs history
- uses: actions/setup-go@v5
  with: { go-version: stable }
- uses: toyz/gw@v0
  with:
    command: affected --since origin/\${{ github.base_ref }}`;

const SYNC = `- uses: toyz/gw@v0
  with:
    command: sync --check   # fail if go.work is stale`;

const INPUTS = [
  {
    name: "command",
    def: 'default: "doctor --strict"',
    desc: "The gw subcommand(s) and flags to run — e.g. sync --check, lint, or affected --since main.",
  },
  {
    name: "version",
    def: 'default: "latest"',
    desc: "gw version to install: a module version/tag (v0.1.1), a branch, or latest.",
  },
  {
    name: "working-directory",
    def: 'default: "."',
    desc: "Directory to run gw in — the workspace root.",
  },
];

function codeWin(title: string, src: string) {
  return (
    <div class="win code">
      <div class="win-bar">
        <span class="dot" />
        <span class="dot" />
        <span class="dot" />
        <span class="win-title">{title}</span>
      </div>
      <div class="win-body">{yamlLines(src)}</div>
    </div>
  );
}

const ciStyles = css`
  .hero {
    display: grid;
    grid-template-columns: 1fr 1.1fr;
    gap: 3.5rem;
    align-items: center;
    padding: 5rem 0 4.5rem;
  }
  @media (max-width: 860px) {
    .hero {
      grid-template-columns: 1fr;
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
    margin: 0 0 1.4rem;
  }
  .lead code {
    font-family: var(--mono);
    font-size: 0.92em;
    color: var(--text);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-family: var(--mono);
    font-size: 0.78rem;
    color: var(--dim);
  }
  .badge b {
    color: var(--green);
    font-weight: 400;
  }

  .inputs {
    display: flex;
    flex-direction: column;
  }
  .input {
    padding: 0.9rem 0;
    border-top: 1px solid var(--border-soft);
  }
  .input:first-child {
    border-top: none;
  }
  .input .head {
    display: flex;
    align-items: baseline;
    gap: 0.7rem;
    flex-wrap: wrap;
  }
  .input code {
    font-family: var(--mono);
    font-size: 0.9rem;
    color: var(--amber);
  }
  .input .def {
    font-family: var(--mono);
    font-size: 0.75rem;
    color: var(--dim);
  }
  .input p {
    margin: 0.3rem 0 0;
    color: var(--dim);
    font-size: 0.9rem;
  }

  .recipe {
    display: grid;
    grid-template-columns: 0.8fr 1.2fr;
    gap: 2.6rem;
    align-items: center;
    margin-bottom: 2.2rem;
  }
  .recipe:last-child {
    margin-bottom: 0;
  }
  @media (max-width: 820px) {
    .recipe {
      grid-template-columns: 1fr;
      gap: 1.4rem;
    }
  }
  .recipe h3 {
    margin: 0 0 0.4rem;
    font-size: 1.05rem;
    font-weight: 600;
  }
  .recipe p {
    margin: 0;
    color: var(--dim);
    font-size: 0.92rem;
  }
  .recipe p code {
    font-family: var(--mono);
    color: var(--teal);
    font-size: 0.88em;
  }
`;

@route("/ci")
@component("page-ci")
@styles(base, ciStyles)
export class PageCI extends LoomElement {
  update() {
    return (
      <div class="wrap">
        <div class="hero">
          <div>
            <div class="kicker mono">
              <b>//</b> GitHub Action
            </div>
            <h1>
              Run gw in <em>CI</em>.
            </h1>
            <p class="lead">
              A composite action — install <code>gw</code> and run any subcommand
              in one step. Default is <code>doctor --strict</code>: fail the build
              on a stale <code>go.work</code>, version drift, or un-hoisted
              replaces.
            </p>
            <span class="badge">
              <loom-icon name="check" size={13} color="var(--green)" /> on the
              GitHub Marketplace
            </span>
          </div>
          {codeWin(".github/workflows/ci.yml", MINIMAL)}
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="layers" size={14} /> Inputs
          </div>
          <div class="inputs">
            {INPUTS.map((i) => (
              <div class="input">
                <div class="head">
                  <code>{i.name}</code>
                  <span class="def">{i.def}</span>
                </div>
                <p>{i.desc}</p>
              </div>
            ))}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="terminal" size={14} /> Recipes
          </div>

          <div class="recipe">
            <div>
              <h3>Affected-only checks</h3>
              <p>
                Test just what a PR touched. <code>fetch-depth: 0</code> gives{" "}
                <code>affected</code> the history it needs to diff.
              </p>
            </div>
            {codeWin("pull_request", AFFECTED)}
          </div>

          <div class="recipe">
            <div>
              <h3>Fail on a stale go.work</h3>
              <p>
                <code>sync --check</code> exits non-zero if the <code>use</code>{" "}
                set drifted from the modules on disk — no writes.
              </p>
            </div>
            {codeWin("step", SYNC)}
          </div>
        </section>
      </div>
    );
  }
}
