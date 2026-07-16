import { LoomElement, component, styles, css } from "@toyz/loom";
import { route } from "@toyz/loom/router";
import { base } from "../styles";
import { codeLines } from "../highlight";
import { GODOC } from "../data";

const HOOK_COMMANDS = [
  "init", "sync", "lint", "doctor", "verify", "build", "test", "vet",
  "generate", "tidy", "run", "list", "graph", "affected", "add", "remove",
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

const FLAGS_SAMPLE = `gwext.Command("greet", "greet someone",
    func(c *gwext.Context) error {
        msg := "hello " + c.String("name")
        if c.Bool("loud") {
            msg = strings.ToUpper(msg)
        }
        fmt.Println(msg)
        return nil
    },
    gwext.Str("name", "world", "who to greet").Alias("n"),
    gwext.Bool("loud", "shout it").Alias("l"))`;

const HOOK_SAMPLE = `// Constants for every built-in command — typo-proof.
gwext.After(gwext.Sync, func(c *gwext.Context) error {
    fmt.Printf("synced %d modules\\n", len(c.Modules))
    return nil
})

// Any command works, including your own custom verbs:
gwext.Before("deploy", func(c *gwext.Context) error {
    return c.Mod("api").Build()
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

const OVERRIDE_SAMPLE = `// Decorate, don't shadow: add a flag, else
// fall through to the real builtin unchanged.
gwext.Override("run", "adds --mode=all",
    func(c *gwext.Context) error {
        if c.Bool("mode-all") {
            return launchAll(c)   // new behavior
        }
        return c.Builtin("run", c.Args...) // original
    },
    gwext.Bool("mode-all", "orchestrated launch")).Passthrough()

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
  .ref {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 1.5rem;
    font-family: var(--mono);
    font-size: 0.9rem;
    color: var(--amber);
    transition: opacity 0.15s;
  }
  .ref:hover {
    opacity: 0.75;
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
            <a class="ref" href={GODOC}>
              <loom-icon name="package" size={15} /> gwext API reference on
              pkg.go.dev
              <loom-icon name="arrow-right" size={13} />
            </a>
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
            <loom-icon name="shield-check" size={14} /> Typed flags
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                Declare flags with <code>gwext.Str/Bool/Int</code> after the
                handler. gw parses them from the user's args; read typed values
                with <code>c.String</code>, <code>c.Bool</code>,{" "}
                <code>c.Int</code> — leftover positionals stay in{" "}
                <code>c.Args</code>.
              </p>
              <p>
                Add short forms with <code>.Alias("n")</code> — either name sets
                the same value. Both <code>--flag value</code> and{" "}
                <code>--flag=value</code> parse. <code>gwext.Strs</code> declares
                a repeatable slice flag (<code>--tag a --tag b</code> or{" "}
                <code>--tag a,b</code>), read with <code>c.Strings</code>. They
                show up in <code>gw &lt;cmd&gt; --help</code> and{" "}
                <code>gw ext list</code>; an unknown flag errors out.
              </p>
            </div>
            {codeWin(".gw/build.go", FLAGS_SAMPLE)}
          </div>
        </section>

        <section>
          <div class="eyebrow">
            <loom-icon name="git-merge" size={14} /> Lifecycle hooks
          </div>
          <div class="grid2">
            <div class="doc">
              <p>
                <code>gwext.Before(cmd, fn)</code> and{" "}
                <code>gwext.After(cmd, fn)</code> run around <b>any</b> command —
                every built-in verb below, plus your own custom commands.
                Constants like <code>gwext.Sync</code> keep the common cases
                typo-proof; a plain string (<code>"deploy"</code>) hooks a custom
                verb.
              </p>
              <div class="events">
                {HOOK_COMMANDS.map((e) => (
                  <span class="evt">{e}</span>
                ))}
              </div>
              <div class="note">
                Multiple hooks per event run in registration order.{" "}
                <code>--dry-run</code>/<code>--check</code> runs skip them. The
                older <code>gwext.Hook("pre-sync", …)</code> string form still
                works but is deprecated.
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
                ignored. To <b>extend</b> one, use <code>gwext.Override</code> —
                decorate, don't shadow: handle your flag, then{" "}
                <code>c.Builtin(name, args...)</code> falls through to the
                original, making the verb a <b>superset</b>. It's surfaced, not
                silent — <code>gw --help</code> and <code>gw ext list</code> flag
                it <code>(overrides builtin)</code>; <code>gwext.Hide</code>
                drops a builtin entirely.
              </p>
              <p>
                Overriding a verb that forwards flags to the go tool
                (<code>build</code>/<code>test</code>/<code>vet</code>/
                <code>run</code>)? Chain <code>.Passthrough()</code> so
                undeclared flags (<code>-p</code>, <code>-race</code>) reach{" "}
                <code>c.Builtin</code> untouched while your own still parse.
              </p>
              <div class="note">
                <b>No recursion.</b> <code>c.Builtin</code> runs the real builtin
                in a child with extensions off, so an override can't re-enter
                itself. Hooks are suppressed on that fall-through; providers and
                env still apply.
              </div>
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
