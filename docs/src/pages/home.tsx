import {
  LoomElement,
  component,
  css,
  inject,
  reactive,
  styles,
} from "@toyz/loom";
import { clipboard } from "@toyz/loom/element";
import { route } from "@toyz/loom/router";
import { COMMAND_GROUPS, FEATURES, INSTALL, REPO, SESSION } from "../data";
import { codeLines } from "../highlight";
import { RepoService } from "../repo";
import { base } from "../styles";

const EXT_SAMPLE = `import "github.com/toyz/gw/gwext"

func main() {
    // Stamp the git SHA in — for free.
    gwext.Provide(gwext.GitStamp("app/version"))

    gwext.Command("boot", "run", func(c *gwext.Context) error {
        return c.Mod("api").Run()
    })

    gwext.Main()
}`;

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

const homeStyles = css`
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
    font-size: clamp(2rem, 6vw, 3.7rem);
    line-height: 1.04;
    letter-spacing: -0.035em;
    font-weight: 680;
    margin: 0 0 1.2rem;
    overflow-wrap: break-word;
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
    font-family: var(--mono);
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
    min-width: 0;
    font-family: var(--mono);
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
    justify-content: center;
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border: 1px solid var(--border);
    background: var(--panel-2);
    color: var(--dim);
    border-radius: 8px;
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s;
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
    flex-wrap: wrap;
    gap: 0.7rem 1.2rem;
    margin-top: 1.6rem;
    font-family: var(--mono);
    font-size: 0.78rem;
    color: var(--dim);
  }
  .badges span {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .badges .rel {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--amber);
    transition: opacity 0.15s;
  }
  .badges .rel:hover {
    opacity: 0.75;
  }

  .features {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1rem;
  }
  @media (max-width: 860px) {
    .features {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    }
  }
  @media (max-width: 560px) {
    .features {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .feat {
    padding: 1.5rem;
    border: 1px solid var(--border-soft);
    border-radius: 12px;
    background: linear-gradient(
      180deg,
      rgba(255, 255, 255, 0.014),
      transparent
    );
    transition:
      border-color 0.2s,
      transform 0.2s;
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

  .panel {
    border: 1px solid var(--border);
    border-radius: 14px;
    background: linear-gradient(
      180deg,
      rgba(255, 255, 255, 0.012),
      transparent
    );
    padding: 2rem 2.2rem;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 0 3.5rem;
  }
  .col > .cmd-group + .cmd-group {
    margin-top: 2rem;
  }
  @media (max-width: 700px) {
    .panel {
      grid-template-columns: minmax(0, 1fr);
      padding: 1.6rem;
      gap: 2rem 0;
    }
  }
  .group-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-family: var(--mono);
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
    font-family: var(--mono);
    font-size: 0.86rem;
    color: var(--amber);
    margin-bottom: 0.1rem;
  }
  .cmd span {
    color: var(--dim);
    font-size: 0.85rem;
    line-height: 1.45;
  }

  .ext {
    display: grid;
    grid-template-columns: minmax(0, 0.78fr) minmax(0, 1.22fr);
    gap: 3rem;
    align-items: center;
  }
  @media (max-width: 860px) {
    .ext {
      grid-template-columns: minmax(0, 1fr);
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
  .more::part(anchor) {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--amber);
    font-size: 0.92rem;
  }
`;

@route("/")
@component("page-home")
@styles(base, homeStyles)
export class PageHome extends LoomElement {
  @reactive accessor copied = false;
  @inject(RepoService) accessor repo!: RepoService;

  @clipboard("write")
  copyInstall() {
    this.copied = true;
    setTimeout(() => (this.copied = false), 1400);
    return INSTALL;
  }

  update() {
    const version = this.repo.tag;
    return (
      <div class="wrap">
        <div class="hero">
          <div>
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
                aria-label={this.copied ? "Copied" : "Copy install command"}
              >
                <loom-icon name={this.copied ? "check" : "copy"} size={15} />
              </button>
            </div>
            <div class="badges">
              <span>
                <loom-icon name="check" size={13} color="var(--green)" /> single
                binary
              </span>
              <span>
                <loom-icon name="check" size={13} color="var(--green)" />{" "}
                zero-config
              </span>
              <span>
                <loom-icon name="check" size={13} color="var(--green)" /> MIT
              </span>
              {version ? (
                <a class="rel" href={REPO + "/releases/latest"}>
                  <loom-icon name="git-branch" size={13} color="var(--amber)" />{" "}
                  {version}
                </a>
              ) : (
                ""
              )}
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
                ),
              )}
              <div class="ln prompt">
                <span class="p">$ </span>
                <span class="cursor" />
              </div>
            </div>
          </div>
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="layers" size={14} /> What it does
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
            <loom-icon name="terminal" size={14} /> Every command
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
            <loom-icon name="package" size={14} /> Compiled extensions
          </div>
          <div class="ext">
            <div>
              <p class="ext-lead">
                A <b>.gw/build.go</b> against the <b>gwext</b> SDK — real
                compiled Go, cached by content hash. Add commands, hooks, and
                build providers; override or hide builtins.
              </p>
              <loom-link to="/extensions" class="more">
                Read the extensions guide{" "}
                <loom-icon name="arrow-right" size={14} color="var(--amber)" />
              </loom-link>
            </div>
            <div class="win code">
              <div class="win-bar">
                <span class="dot" />
                <span class="dot" />
                <span class="dot" />
                <span class="win-title">.gw/build.go</span>
              </div>
              <div class="win-body">{codeLines(EXT_SAMPLE)}</div>
            </div>
          </div>
        </section>
      </div>
    );
  }
}
