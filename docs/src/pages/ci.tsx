import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import { yamlLines } from "../highlight";
import { latestVersion, bindVersion } from "../store";

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

// inputs builds the action's input cards. The version example is interpolated
// from the live release tag (shared store) rather than hardcoded — so it's
// always current, and simply omits the parenthetical until the tag resolves.
function inputs(tag: string) {
  const example = tag ? ` (${tag})` : "";
  return [
    {
      name: "command",
      icon: "terminal",
      default: "doctor --strict",
      desc: "The gw subcommand(s) and flags to run — sync --check, lint, affected --since main, …",
    },
    {
      name: "version",
      icon: "git-branch",
      default: "latest",
      desc: `gw version to install: a module tag${example}, a branch, or latest.`,
    },
    {
      name: "working-directory",
      icon: "package",
      default: ".",
      desc: "Directory to run gw in — your workspace root.",
    },
  ];
}

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
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }
  @media (max-width: 760px) {
    .inputs {
      grid-template-columns: 1fr;
    }
  }
  .input {
    display: flex;
    flex-direction: column;
    padding: 1.4rem;
    border: 1px solid var(--border-soft);
    border-radius: 12px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.014), transparent);
    transition: border-color 0.2s, transform 0.2s;
  }
  .input:hover {
    border-color: var(--border);
    transform: translateY(-2px);
  }
  .input-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
  }
  .ichip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: 9px;
    background: color-mix(in srgb, var(--c) 15%, transparent);
    color: var(--c);
    border: 1px solid color-mix(in srgb, var(--c) 30%, transparent);
  }
  .input:nth-child(1) .ichip {
    --c: var(--amber);
  }
  .input:nth-child(2) .ichip {
    --c: var(--teal);
  }
  .input:nth-child(3) .ichip {
    --c: var(--violet);
  }
  .opt {
    font-family: var(--mono);
    font-size: 0.64rem;
    text-transform: uppercase;
    letter-spacing: 0.09em;
    color: var(--dim);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 0.12rem 0.42rem;
  }
  .iname {
    font-family: var(--mono);
    font-size: 0.98rem;
    color: var(--amber);
  }
  .idefault {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin: 0.7rem 0 0.95rem;
  }
  .idefault .dlabel {
    font-family: var(--mono);
    font-size: 0.72rem;
    color: var(--dim);
  }
  .idefault code {
    font-family: var(--mono);
    font-size: 0.8rem;
    color: var(--teal);
    background: var(--panel-2);
    border: 1px solid var(--border-soft);
    border-radius: 6px;
    padding: 0.15rem 0.45rem;
    white-space: nowrap;
    overflow-x: auto;
    max-width: 100%;
  }
  .input p {
    margin: 0;
    color: var(--dim);
    font-size: 0.88rem;
    line-height: 1.5;
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
  // Re-render when the shared release tag lands, so the version example is
  // always the current release — never hardcoded.
  firstUpdated() {
    bindVersion(this);
  }

  update() {
    const rows = inputs(latestVersion.value);
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
            {rows.map((i) => (
              <div class="input">
                <div class="input-top">
                  <span class="ichip">
                    <loom-icon name={i.icon} size={17} color="currentColor" />
                  </span>
                  <span class="opt">optional</span>
                </div>
                <code class="iname">{i.name}</code>
                <div class="idefault">
                  <span class="dlabel">default</span>
                  <code>{i.default}</code>
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
