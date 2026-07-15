import { LoomElement, component, reactive, css, styles } from "@toyz/loom";
import { clipboard } from "@toyz/loom/element";
import { FEATURES, COMMAND_GROUPS, SESSION } from "./data";

const REPO = "https://github.com/toyz/gw";
const INSTALL = "go install github.com/toyz/gw@latest";

const EXT_SAMPLE = `import "github.com/toyz/gw/gwext"

func main() {
    // Stamp the git SHA in — for free.
    gwext.Provide(gwext.GitStamp("app/version"))

    // Custom command:  gw boot
    gwext.Command("boot", "run", func(c *gwext.Context) error {
        return c.Mod("api").Run()
    })

    gwext.Main()
}`;

const CAPS = ["commands", "hooks", "build providers", "override / hide"];

const GO_KW = new Set([
  "import", "func", "return", "package", "var", "const", "type", "range",
  "for", "if", "else", "go", "defer", "map", "struct", "interface", "chan",
]);

// hlGo tokenizes one line of Go into colored spans: strings, keywords, function
// calls, comments. The samples are controlled (no `//` inside strings), so a
// small regex is enough — no full parser needed.
function hlGo(line: string) {
  if (line.trim() === "") return " ";
  const ci = line.indexOf("//");
  const code = ci >= 0 ? line.slice(0, ci) : line;
  const comment = ci >= 0 ? line.slice(ci) : "";
  const out: unknown[] = [];
  const re = /("(?:[^"\\]|\\.)*")|([A-Za-z_][A-Za-z0-9_]*)|(\s+)|([^\sA-Za-z_"]+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(code))) {
    if (m[1]) out.push(<span class="s">{m[1]}</span>);
    else if (m[2]) {
      if (GO_KW.has(m[2])) out.push(<span class="k">{m[2]}</span>);
      else if (code[re.lastIndex] === "(") out.push(<span class="fn">{m[2]}</span>);
      else out.push(m[2]);
    } else if (m[3]) out.push(m[3]);
    else if (m[4]) out.push(<span class="pu">{m[4]}</span>);
  }
  if (comment) out.push(<span class="cm">{comment}</span>);
  return out;
}

function cmdGroup(g: (typeof COMMAND_GROUPS)[number]) {
  return (
    <div class="cmd-group">
      <div class="group-label">
        <loom-icon name="arrow-right" size={12} color="var(--teal)" />
        {g.label}
      </div>
      {g.items.map((c) => (
        <div class="cmd">
          <code>{c.name}</code>
          <span>{c.desc}</span>
        </div>
      ))}
    </div>
  );
}

const siteStyles = css`
  :host {
    --bg: #0a0c0f;
    --panel: #0f1319;
    --panel-2: #12161d;
    --border: #212832;
    --border-soft: #191e26;
    --text: #e8edf2;
    --dim: #8b949e;
    --amber: #e3a44f;
    --green: #5fcf90;
    --teal: #63c8cf;
    --violet: #b492f0;
    --cyan: #6ab0f2;
    --rose: #ec8aa0;
    display: block;
    min-height: 100vh;
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica,
      Arial, sans-serif;
    line-height: 1.6;
    font-size: 16px;
  }
  .mono {
    font-family: "SF Mono", ui-monospace, "JetBrains Mono", Menlo, monospace;
  }
  a {
    color: inherit;
    text-decoration: none;
  }
  .wrap {
    max-width: 1120px;
    margin: 0 auto;
    padding: 0 2rem;
  }
  loom-icon {
    flex-shrink: 0;
  }

  /* header */
  header {
    position: sticky;
    top: 0;
    z-index: 10;
    backdrop-filter: blur(10px);
    background: rgba(10, 12, 15, 0.72);
    border-bottom: 1px solid var(--border-soft);
  }
  .header-in {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.9rem 0;
  }
  .logo {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-weight: 700;
    font-size: 1.05rem;
    letter-spacing: -0.02em;
    color: var(--text);
  }
  .logo loom-icon {
    color: var(--amber);
  }
  .gh {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--dim);
    font-size: 0.9rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    transition: color 0.15s, border-color 0.15s;
  }
  .gh:hover {
    color: var(--text);
    border-color: #2c343e;
  }

  /* hero */
  .hero {
    display: grid;
    grid-template-columns: 1.02fr 1.08fr;
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
    max-width: 46ch;
    margin: 0 0 2rem;
  }
  .lead code {
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.92em;
    color: var(--text);
  }

  .install {
    display: inline-flex;
    align-items: center;
    gap: 0.9rem;
    padding: 0.5rem 0.55rem 0.5rem 1rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: rgba(15, 19, 25, 0.9);
    max-width: 100%;
  }
  .install code {
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.9rem;
    white-space: nowrap;
    overflow-x: auto;
    color: var(--text);
  }
  .install code i {
    color: var(--teal);
    font-style: normal;
  }
  .cp {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    border: 1px solid var(--border);
    background: var(--panel-2);
    color: var(--dim);
    border-radius: 7px;
    padding: 0.35rem 0.6rem;
    font-size: 0.78rem;
    cursor: pointer;
    transition: color 0.15s, border-color 0.15s;
    font-family: inherit;
  }
  .cp:hover {
    color: var(--text);
    border-color: #2c343e;
  }
  .cp.ok {
    color: var(--green);
    border-color: rgba(95, 207, 144, 0.5);
  }
  .badges {
    display: flex;
    gap: 1.2rem;
    margin-top: 1.6rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.78rem;
    color: var(--dim);
  }
  .badges span {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .badges b {
    color: var(--green);
    font-weight: 400;
  }

  /* window (terminal + code share chrome) */
  .win {
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--panel);
    overflow: hidden;
    box-shadow: 0 24px 60px -34px rgba(0, 0, 0, 0.85),
      0 -1px 0 0 rgba(255, 255, 255, 0.03) inset;
  }
  .win-bar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.7rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    background: var(--panel-2);
  }
  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;
    background: #2b323b;
  }
  .win-title {
    margin-left: 0.5rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.76rem;
    color: var(--dim);
  }
  .win-body {
    padding: 1.15rem 1.35rem 1.4rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.85rem;
    line-height: 1.85;
    overflow-x: auto;
  }
  .ln {
    white-space: pre;
  }
  .ln .p {
    color: var(--amber);
  }
  .ln.prompt {
    color: var(--text);
    margin-top: 0.55rem;
  }
  .ln.prompt:first-child {
    margin-top: 0;
  }
  .ln.add,
  .ln.ok {
    color: var(--green);
  }
  .ln.path {
    color: var(--teal);
  }
  .ln.dim {
    color: var(--dim);
  }
  .cursor {
    display: inline-block;
    width: 8px;
    height: 1em;
    background: var(--amber);
    vertical-align: text-bottom;
    animation: blink 1.1s steps(2, start) infinite;
  }
  @keyframes blink {
    50% {
      opacity: 0;
    }
  }

  /* sections */
  section {
    padding: 3.75rem 0;
    border-top: 1px solid var(--border-soft);
  }
  .eyebrow {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.76rem;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    color: var(--dim);
    margin: 0 0 2rem;
  }
  .eyebrow loom-icon {
    color: var(--amber);
  }

  /* feature cards */
  .features {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }
  @media (max-width: 860px) {
    .features {
      grid-template-columns: 1fr 1fr;
    }
  }
  @media (max-width: 560px) {
    .features {
      grid-template-columns: 1fr;
    }
  }
  .feat {
    padding: 1.5rem;
    border: 1px solid var(--border-soft);
    border-radius: 12px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.014), transparent);
    transition: border-color 0.2s, transform 0.2s;
  }
  .feat:hover {
    border-color: var(--border);
    transform: translateY(-2px);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 38px;
    height: 38px;
    border-radius: 10px;
    margin-bottom: 1rem;
    background: color-mix(in srgb, var(--c) 15%, transparent);
    color: var(--c);
    border: 1px solid color-mix(in srgb, var(--c) 30%, transparent);
  }
  .feat:nth-child(1) .chip {
    --c: var(--amber);
  }
  .feat:nth-child(2) .chip {
    --c: var(--green);
  }
  .feat:nth-child(3) .chip {
    --c: var(--violet);
  }
  .feat:nth-child(4) .chip {
    --c: var(--teal);
  }
  .feat:nth-child(5) .chip {
    --c: var(--rose);
  }
  .feat:nth-child(6) .chip {
    --c: var(--cyan);
  }
  .feat h3 {
    margin: 0 0 0.45rem;
    font-size: 1.05rem;
    font-weight: 600;
  }
  .feat p {
    margin: 0;
    color: var(--dim);
    font-size: 0.9rem;
    line-height: 1.55;
  }

  /* commands panel */
  .panel {
    border: 1px solid var(--border);
    border-radius: 14px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.012), transparent);
    padding: 2rem 2.2rem;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 3.5rem;
  }
  .col > .cmd-group + .cmd-group {
    margin-top: 2rem;
  }
  @media (max-width: 700px) {
    .panel {
      grid-template-columns: 1fr;
      padding: 1.6rem;
      gap: 2rem 0;
    }
  }
  .group-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.13em;
    color: var(--teal);
    margin: 0 0 0.8rem;
  }
  .cmd {
    padding: 0.5rem 0;
    border-top: 1px solid var(--border-soft);
  }
  .cmd code {
    display: block;
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.86rem;
    color: var(--amber);
    margin-bottom: 0.1rem;
  }
  .cmd span {
    color: var(--dim);
    font-size: 0.85rem;
    line-height: 1.45;
  }

  /* extensions */
  .ext {
    display: grid;
    grid-template-columns: 0.78fr 1.22fr;
    gap: 3rem;
    align-items: center;
  }
  @media (max-width: 860px) {
    .ext {
      grid-template-columns: 1fr;
      gap: 2rem;
    }
  }
  .ext-lead {
    color: #aeb6bf;
    margin: 0 0 1.5rem;
  }
  .ext-lead b {
    color: var(--text);
    font-weight: 600;
  }
  .caps {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .cap {
    font-family: "SF Mono", ui-monospace, monospace;
    font-size: 0.76rem;
    color: var(--dim);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.25rem 0.7rem;
  }
  .code .win-body {
    line-height: 1.7;
    font-size: 0.8rem;
  }
  .cl {
    white-space: pre;
    color: #cdd5dd;
  }
  .win-body .k {
    color: var(--violet);
  }
  .win-body .s {
    color: #9ecf7f;
  }
  .win-body .fn {
    color: var(--cyan);
  }
  .win-body .cm {
    color: #61707c;
  }
  .win-body .pu {
    color: #7f8a95;
  }

  footer {
    border-top: 1px solid var(--border-soft);
  }
  .footer-in {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 2.5rem 0 3.5rem;
    color: var(--dim);
    font-size: 0.86rem;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .footer-in a:hover {
    color: var(--amber);
  }
`;

@component("gw-site")
@styles(siteStyles)
export class GwSite extends LoomElement {
  @reactive accessor copied = false;

  // @clipboard("write") copies this method's return value (execCommand fallback).
  @clipboard("write")
  copyInstall() {
    this.copied = true;
    setTimeout(() => (this.copied = false), 1400);
    return INSTALL;
  }

  update() {
    return (
      <div>
        <header>
          <div class="wrap header-in">
            <span class="logo">
              <loom-icon name="git-branch" size={18} />
              gw
            </span>
            <a class="gh" href={REPO}>
              <loom-icon name="github" size={17} />
              GitHub
            </a>
          </div>
        </header>

        <div class="wrap">
          <div class="hero">
            <div class="hero-copy">
              <div class="kicker mono">
                <b>//</b> the go.work workspace manager
              </div>
              <h1>
                Go workspaces that actually <em>scale</em>.
              </h1>
              <p class="lead">
                One binary to generate and maintain <code>go.work</code>, lint
                dependency versions across modules, run the go tool everywhere,
                and extend it all with compiled Go.
              </p>
              <div class="install">
                <code>
                  <i>$</i> {INSTALL}
                </code>
                <button
                  class={this.copied ? "cp ok" : "cp"}
                  onClick={() => this.copyInstall()}
                >
                  <loom-icon name={this.copied ? "check" : "copy"} size={14} />
                  {this.copied ? "copied" : "copy"}
                </button>
              </div>
              <div class="badges">
                <span>
                  <loom-icon name="check" size={13} color="var(--green)" />
                  single binary
                </span>
                <span>
                  <loom-icon name="check" size={13} color="var(--green)" />
                  zero-config
                </span>
                <span>
                  <loom-icon name="check" size={13} color="var(--green)" />
                  MIT
                </span>
              </div>
            </div>

            <div class="win term">
              <div class="win-bar">
                <span class="dot" />
                <span class="dot" />
                <span class="dot" />
                <span class="win-title">workspace — gw</span>
              </div>
              <div class="win-body">
                {SESSION.map((l) =>
                  l.kind === "prompt" ? (
                    <div class="ln prompt">
                      <span class="p">$ </span>
                      {l.text}
                    </div>
                  ) : (
                    <div class={"ln " + l.kind}>{l.text}</div>
                  )
                )}
                <div class="ln prompt">
                  <span class="p">$ </span>
                  <span class="cursor" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="wrap">
          <section>
            <div class="eyebrow">
              <loom-icon name="layers" size={14} />
              What it does
            </div>
            <div class="features">
              {FEATURES.map((f) => (
                <div class="feat">
                  <span class="chip">
                    <loom-icon name={f.icon} size={19} color="currentColor" />
                  </span>
                  <h3>{f.title}</h3>
                  <p>{f.body}</p>
                </div>
              ))}
            </div>
          </section>

          <section>
            <div class="eyebrow">
              <loom-icon name="terminal" size={14} />
              Every command
            </div>
            <div class="panel">
              <div class="col">
                {cmdGroup(COMMAND_GROUPS[0])}
                {cmdGroup(COMMAND_GROUPS[2])}
              </div>
              <div class="col">
                {cmdGroup(COMMAND_GROUPS[1])}
                {cmdGroup(COMMAND_GROUPS[3])}
              </div>
            </div>
          </section>

          <section>
            <div class="eyebrow">
              <loom-icon name="package" size={14} />
              Compiled extensions
            </div>
            <div class="ext">
              <div>
                <p class="ext-lead">
                  A <b>.gw/build.go</b> against the <b>gwext</b> SDK — real
                  compiled Go, cached by content hash. Think Cargo's build.rs,
                  but you never write the git plumbing.
                </p>
                <div class="caps">
                  {CAPS.map((c) => (
                    <span class="cap">{c}</span>
                  ))}
                </div>
              </div>
              <div class="win code">
                <div class="win-bar">
                  <span class="dot" />
                  <span class="dot" />
                  <span class="dot" />
                  <span class="win-title">.gw/build.go</span>
                </div>
                <div class="win-body">
                  {EXT_SAMPLE.split("\n").map((line) => (
                    <div class="cl">{hlGo(line)}</div>
                  ))}
                </div>
              </div>
            </div>
          </section>
        </div>

        <footer>
          <div class="wrap footer-in">
            <a href={REPO}>github.com/toyz/gw · MIT</a>
            <a href="https://toyz.github.io/loom/">built with @toyz/loom</a>
          </div>
        </footer>
      </div>
    );
  }
}
