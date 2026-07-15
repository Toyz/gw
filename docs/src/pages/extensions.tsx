import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import { codeLines } from "../highlight";

const HOOK_EVENTS = [
  "pre-sync", "post-sync", "pre-lint", "post-lint", "pre-build", "post-build",
  "pre-test", "post-test", "pre-vet", "post-vet", "pre-generate",
  "post-generate", "pre-tidy", "post-tidy", "pre-run", "post-run",
];

const SCAFFOLD = `$ gw ext init      # scaffold .gw/build.go + .gw/go.mod
$ gw ext build     # compile (cached by content hash)
$ gw ext list      # show commands, hooks, providers`;

const CMD_SAMPLE = `gwext.Command("boot", "build in order",
    func(c *gwext.Context) error {
        for _, p := range []string{"api", "gateway"} {
            if err := c.Mod(p).Build(); err != nil {
                return err
            }
        }
        return nil
    })`;

const HOOK_SAMPLE = `gwext.Hook("post-sync", func(c *gwext.Context) error {
    fmt.Printf("synced %d modules\\n", len(c.Modules))
    return nil
})`;

const PROVIDE_SAMPLE = `// Prebuilt: stamp git metadata into your binary.
gwext.Provide(gwext.GitStamp("example.com/app/version"))

// Or compute your own env / -ldflags -X / build tags:
gwext.Provide(func(c *gwext.Context) (gwext.BuildInfo, error) {
    return gwext.BuildInfo{
        Env:  map[string]string{"BUILD_ENV": "ci"},
        Tags: []string{"prod"},
    }, nil
})`;

const EACH_SAMPLE = `// Only the gateway gets the "edge" tag.
gwext.ProvideEach(
    func(c *gwext.Context, m gwext.Module) (gwext.BuildInfo, error) {
        if m.Path == "gateway" {
            return gwext.BuildInfo{Tags: []string{"edge"}}, nil
        }
        return gwext.BuildInfo{}, nil
    })`;

const OVERRIDE_SAMPLE = `// Replace a builtin — gw removes the shadowed one.
gwext.Override("test", "with a db",
    func(c *gwext.Context) error {
        startDB(); defer stopDB()
        return c.Mod("api").Test()
    })

// Or drop a builtin entirely.
gwext.Hide("generate")`;

const TOOL_SAMPLE = `// Any toolchain, in a module's directory.
gwext.Command("web", "dev server",
    func(c *gwext.Context) error {
        return c.Mod("web").Tool("yarn").Run("dev")
    })`;

function codeWin(title: string, src: string) {
  return (
    <div class="win code">
      <div class="win-bar">
        <span class="dot" />
        <span class="dot" />
        <span class="dot" />
        <span class="win-title">{title}</span>
      </div>
      <div class="win-body">{codeLines(src)}</div>
    </div>
  );
}

const extStyles = css`
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
    max-width: 48ch;
    margin: 0;
  }
  .lead b {
    color: var(--text);
    font-weight: 600;
  }

  .grid2 {
    display: grid;
    grid-template-columns: 0.92fr 1.08fr;
    gap: 2.6rem;
    align-items: start;
  }
  @media (max-width: 820px) {
    .grid2 {
      grid-template-columns: 1fr;
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

  .events {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-top: 1.2rem;
  }
  .evt {
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
`;

@route("/extensions")
@component("page-extensions")
@styles(base, extStyles)
export class PageExtensions extends LoomElement {
  update() {
    return (
      <div class="wrap">
        <div class="hero">
          <div>
            <div class="kicker mono">
              <b>//</b> gwext SDK
            </div>
            <h1>
              Extend gw in <em>compiled Go</em>.
            </h1>
            <p class="lead">
              A <b>.gw/build.go</b> against the <b>gwext</b> SDK is real compiled
              Go — cached by content hash, rebuilt only when its sources change.
              Add commands, hook lifecycle events, contribute build settings,
              and reshape the command tree.
            </p>
          </div>
          <div class="win term">
            <div class="win-bar">
              <span class="dot" />
              <span class="dot" />
              <span class="dot" />
              <span class="win-title">scaffold</span>
            </div>
            <div class="win-body">
              {SCAFFOLD.split("\n").map((l) => {
                const body = l.replace(/^\$ /, "");
                const hi = body.indexOf("#");
                const cmd = hi >= 0 ? body.slice(0, hi) : body;
                const comment = hi >= 0 ? body.slice(hi) : "";
                return (
                  <div class="ln prompt">
                    <span class="p">$ </span>
                    {cmd}
                    {comment ? <span class="cm">{comment}</span> : ""}
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        <section>
          <div class="eyebrow">
            <loom-icon name="terminal" size={14} /> Custom commands
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gwext.Command(name, short, fn)</code> adds a{" "}
                <code>gw &lt;name&gt;</code> subcommand. The handler gets a{" "}
                <code>*gwext.Context</code> with the workspace root and every
                module — orchestrate builds, fan out, or drive any tool.
              </p>
              <p>
                <code>c.Mod(path)</code> is a typed handle:{" "}
                <code>.Build() .Test() .Run() .Vet() .Generate() .Tidy()</code>,
                plus <code>.Tool(bin)</code> for anything else.
              </p>
            </div>
            {codeWin(".gw/build.go", CMD_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-merge" size={14} /> Lifecycle hooks
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gwext.Hook(event, fn)</code> runs on gw's{" "}
                <code>pre-</code>/<code>post-</code> events. Multiple hooks per
                event run in registration order.
              </p>
              <div class="events">
                {HOOK_EVENTS.map((e) => (
                  <span class="evt">{e}</span>
                ))}
              </div>
            </div>
            {codeWin(".gw/build.go", HOOK_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="package" size={14} /> Build providers
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gwext.Provide</code> computes <b>env</b>,{" "}
                <b>-ldflags -X</b> vars, and <b>build tags</b> at run time — gw's
                take on a Cargo build script's <code>rustc-env</code>/
                <code>rustc-cfg</code>. Prebuilt <code>GitStamp</code>/
                <code>GitEnv</code> ship in the SDK.
              </p>
              <div class="note">
                <b>Precedence.</b> Provider env layers between config and{" "}
                <code>--env</code>: config &lt; provider &lt; CLI. Tags/vars are
                woven into <code>build</code>/<code>test</code>/<code>vet</code>.
              </div>
            </div>
            {codeWin(".gw/build.go", PROVIDE_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-branch" size={14} /> Per-module providers
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gwext.ProvideEach</code> runs once per module and applies
                its <code>BuildInfo</code> to that module alone — scope a tag,
                env var, or stamp to just the services that need it.
              </p>
              <p>
                Global <code>Provide</code> output still applies everywhere;
                per-module values layer on top.
              </p>
            </div>
            {codeWin(".gw/build.go", EACH_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="layers" size={14} /> Override &amp; hide
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                A plain <code>Command</code> that collides with a builtin is
                ignored. To <b>replace</b> one, use <code>gwext.Override</code> —
                gw removes the shadowed builtin. <code>gwext.Hide</code> drops
                builtins from the tree entirely.
              </p>
            </div>
            {codeWin(".gw/build.go", OVERRIDE_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="package" size={14} /> Other toolchains
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                Not just Go. <code>c.Mod(path).Tool(bin)</code> binds any
                executable and runs it in that module's directory — npm, yarn,
                cargo, docker, deno. <code>.Start(...)</code> launches long-lived
                processes like dev servers.
              </p>
            </div>
            {codeWin(".gw/build.go", TOOL_SAMPLE)}
          </div>
        </section>
      </div>
    );
  }
}
