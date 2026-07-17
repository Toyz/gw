import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";

const REGISTER = `$ claude mcp add gw -- gw mcp
$ # the agent now has gw_list, gw_lint,
$ # gw_graph, gw_affected, gw_sync, gw_test`;

const TOOLS = [
  {
    name: "gw_list",
    icon: "package",
    kind: "read",
    desc: "Every module with its path, directory, go version, and direct requires — as JSON.",
  },
  {
    name: "gw_doctor",
    icon: "shield-check",
    kind: "read",
    desc: "Workspace health: stale go.work, orphaned use entries, un-hoisted replaces, version drift.",
  },
  {
    name: "gw_lint",
    icon: "sliders",
    kind: "read",
    desc: "Dependencies pinned at different versions across modules, plus go/toolchain drift.",
  },
  {
    name: "gw_graph",
    icon: "git-merge",
    kind: "read",
    desc: "The intra-workspace dependency DAG: nodes and directed A→B edges.",
  },
  {
    name: "gw_affected",
    icon: "git-branch",
    kind: "read",
    desc: "Changed files since a git ref → the transitively impacted modules. Selective CI.",
  },
  {
    name: "gw_sync",
    icon: "zap",
    kind: "act",
    desc: "Regenerate go.work from the modules on disk. check mode reports drift without writing.",
  },
  {
    name: "gw_test",
    icon: "terminal",
    kind: "act",
    desc: "Run go test across every module and return the combined output and pass/fail summary.",
  },
];

function term(title: string, src: string) {
  return (
    <div class="win term">
      <div class="win-bar">
        <span class="dot" />
        <span class="dot" />
        <span class="dot" />
        <span class="win-title">{title}</span>
      </div>
      <div class="win-body">
        {src.split("\n").map((l) => {
          const body = l.replace(/^\$ /, "");
          const isComment = body.startsWith("#");
          return (
            <div class="ln prompt">
              <span class="p">$ </span>
              {isComment ? <span class="cm">{body}</span> : body}
            </div>
          );
        })}
      </div>
    </div>
  );
}

const mcpStyles = css`
  .hero {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr);
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
    max-width: 50ch;
    margin: 0;
  }
  .lead code {
    font-family: var(--mono);
    font-size: 0.92em;
    color: var(--text);
  }

  .tools {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1rem;
  }
  @media (max-width: 760px) {
    .tools {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .tool {
    display: flex;
    flex-direction: column;
    padding: 1.3rem;
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
  .tool:hover {
    border-color: var(--border);
    transform: translateY(-2px);
  }
  .tool-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.9rem;
  }
  .tchip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: 9px;
    background: color-mix(in srgb, var(--c) 15%, transparent);
    color: var(--c);
    border: 1px solid color-mix(in srgb, var(--c) 30%, transparent);
    --c: var(--teal);
  }
  .tool.act .tchip {
    --c: var(--amber);
  }
  .kind {
    font-family: var(--mono);
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.09em;
    color: var(--dim);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 0.12rem 0.42rem;
  }
  .tname {
    font-family: var(--mono);
    font-size: 0.98rem;
    color: var(--amber);
    margin-bottom: 0.5rem;
  }
  .tool p {
    margin: 0;
    color: var(--dim);
    font-size: 0.86rem;
    line-height: 1.5;
  }

  .recipe {
    display: grid;
    grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr);
    gap: 2.6rem;
    align-items: center;
  }
  @media (max-width: 820px) {
    .recipe {
      grid-template-columns: minmax(0, 1fr);
      gap: 1.4rem;
    }
  }
  .recipe h3 {
    margin: 0 0 0.5rem;
    font-size: 1.05rem;
    font-weight: 600;
  }
  .recipe p {
    margin: 0 0 0.8rem;
    color: #aeb6bf;
    font-size: 0.95rem;
  }
  .recipe p:last-child {
    margin-bottom: 0;
  }
  .recipe code {
    font-family: var(--mono);
    color: var(--teal);
    font-size: 0.88em;
  }
  .note {
    margin-top: 1.4rem;
    padding: 0.9rem 1.1rem;
    border: 1px solid var(--border-soft);
    border-left: 2px solid var(--teal);
    border-radius: 8px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.012), transparent);
    color: var(--dim);
    font-size: 0.9rem;
  }
  .note b {
    color: var(--text);
    font-weight: 600;
  }
`;

@route("/mcp")
@component("page-mcp")
@styles(base, mcpStyles)
export class PageMCP extends LoomElement {
  update() {
    return (
      <div class="wrap">
        <div class="hero">
          <div>
            <div class="kicker mono">
              <b>//</b> MCP server
            </div>
            <h1>
              Drive the workspace from an <em>agent</em>.
            </h1>
            <p class="lead">
              <b>gw mcp</b> is a Model Context Protocol stdio server. Register it
              once and an agent can map your modules, catch version drift, run
              only what a diff touched, and sync <code>go.work</code> — no
              shelling out, structured JSON in reply.
            </p>
          </div>
          {term("register", REGISTER)}
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="layers" size={14} /> Tools
          </div>
          <div class="tools">
            {TOOLS.map((t) => (
              <div class={"tool " + (t.kind === "act" ? "act" : "read")}>
                <div class="tool-top">
                  <span class="tchip">
                    <loom-icon name={t.icon} size={17} color="currentColor" />
                  </span>
                  <span class="kind">
                    {t.kind === "act" ? "action" : "read-only"}
                  </span>
                </div>
                <code class="tname">{t.name}</code>
                <p>{t.desc}</p>
              </div>
            ))}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-branch" size={14} /> Agent-driven selective CI
          </div>
          <div class="recipe">
            <div>
              <h3>Test only what changed</h3>
              <p>
                The agent calls <code>gw_affected</code> with a base ref, gets
                back the modules a diff actually touches (plus everything that
                depends on them), then runs <code>gw_test</code> scoped to that
                set — instead of the whole workspace.
              </p>
              <p>
                Read-only tools (<code>list</code>, <code>doctor</code>,{" "}
                <code>lint</code>, <code>graph</code>, <code>affected</code>)
                call gw's workspace engine directly and answer in JSON; action
                tools (<code>sync</code>, <code>test</code>) run gw, so they
                inherit hooks and build providers.
              </p>
            </div>
            {term(
              "agent loop",
              `$ gw_affected  { since: "main" }
$ # -> impacted: [api, gateway]
$ gw_test  { packages: "./..." }
$ # -> 2 modules, 0 failed`,
            )}
          </div>
          <div class="note">
            <b>Every tool takes an optional root</b> — it defaults to the nearest{" "}
            <code>go.work</code>, so the agent doesn't have to know where the
            workspace lives.
          </div>
        </section>
      </div>
    );
  }
}
