import {
  LoomElement,
  component,
  inject,
  mount,
  on,
  reactive,
  styles,
} from "@toyz/loom";
import { media } from "@toyz/loom/element";
import "@toyz/loom/router"; // registers <loom-outlet> and <loom-link>
import { RouteChanged } from "@toyz/loom/router";
import { REPO, fmtCount } from "./data";
import { RepoService } from "./repo";
import { base } from "./styles";

// navItems renders the nav links + version pill + GitHub star button. Shared by
// the desktop bar and the mobile dropdown — only one is rendered at a time.
function navItems(
  active: (to: string) => string,
  version: string,
  stars: number,
) {
  return [
    <loom-link to="/" class={active("/")}>
      Overview
    </loom-link>,
    <loom-link to="/recipes" class={active("/recipes")}>
      Recipes
    </loom-link>,
    <loom-link to="/extensions" class={active("/extensions")}>
      Extensions
    </loom-link>,
    <loom-link to="/config" class={active("/config")}>
      Config
    </loom-link>,
    <loom-link to="/ci" class={active("/ci")}>
      CI
    </loom-link>,
    <loom-link to="/mcp" class={active("/mcp")}>
      MCP
    </loom-link>,
    version ? (
      <a class="ver" href={REPO + "/releases/latest"}>
        {version}
      </a>
    ) : (
      ""
    ),
    <a class="gh" href={REPO}>
      <loom-icon name="github" size={16} /> GitHub
      {stars > 0 ? (
        <span class="stars">
          <loom-icon name="star" size={13} /> {fmtCount(stars)}
        </span>
      ) : (
        ""
      )}
    </a>,
  ];
}

// mobileMenu renders the dropdown panel: full-width nav links, then a divider
// and a row of meta chips (GitHub, star count, release tag).
function mobileMenu(
  active: (to: string) => string,
  version: string,
  stars: number,
) {
  return (
    <div class="nav-drop">
      <loom-link to="/" class={"m-link " + active("/")}>
        Overview
      </loom-link>
      <loom-link to="/recipes" class={"m-link " + active("/recipes")}>
        Recipes
      </loom-link>
      <loom-link to="/extensions" class={"m-link " + active("/extensions")}>
        Extensions
      </loom-link>
      <loom-link to="/config" class={"m-link " + active("/config")}>
        Config
      </loom-link>
      <loom-link to="/ci" class={"m-link " + active("/ci")}>
        CI
      </loom-link>
      <loom-link to="/mcp" class={"m-link " + active("/mcp")}>
        MCP
      </loom-link>
      <div class="m-meta">
        <a class="m-chip" href={REPO}>
          <loom-icon name="github" size={15} /> GitHub
        </a>
        {stars > 0 ? (
          <a class="m-chip" href={REPO + "/stargazers"}>
            <loom-icon name="star" size={14} /> {fmtCount(stars)}
          </a>
        ) : (
          ""
        )}
        {version ? (
          <a class="m-chip" href={REPO + "/releases/latest"}>
            <loom-icon name="git-branch" size={14} /> {version}
          </a>
        ) : (
          ""
        )}
      </div>
    </div>
  );
}

// Shell: sticky header with route-aware nav (a hamburger on mobile), the routed
// outlet, and a footer.
@component("gw-site")
@styles(base)
export class GwSite extends LoomElement {
  @reactive accessor path = "/";
  @reactive accessor menuOpen = false;
  // Reactive breakpoint — flips the header between the inline bar and a
  // hamburger + dropdown, updating live on resize.
  @media("(max-width: 640px)") accessor isMobile = false;
  // Singleton service; its start() ran (and awaited the fetches) before this
  // component ever rendered, so tag/stars are already populated below.
  @inject(RepoService) accessor repo!: RepoService;

  // Read the initial route on connect (before first paint), so the global
  // location read stays out of the constructor.
  @mount
  readInitialPath() {
    this.path = currentPath();
  }

  @on(RouteChanged)
  onRoute(e: RouteChanged) {
    this.path = e.path;
    this.menuOpen = false; // close the mobile menu on navigation
  }

  update() {
    const active = (to: string) => (this.path === to ? "active" : "");
    const version = this.repo.tag;
    const stars = this.repo.stars;
    return (
      <div>
        <header>
          <div class="wrap header-in">
            <loom-link to="/" class="logo">
              <loom-icon name="git-branch" size={18} />
              <b>gw</b>
              <span class="tag">go.work manager</span>
            </loom-link>
            {this.isMobile ? (
              <button
                class="burger"
                aria-label="Menu"
                aria-expanded={this.menuOpen ? "true" : "false"}
                onClick={() => (this.menuOpen = !this.menuOpen)}
              >
                <loom-icon name={this.menuOpen ? "x" : "menu"} size={20} />
              </button>
            ) : (
              <div class="nav">{navItems(active, version, stars)}</div>
            )}
          </div>
          {this.isMobile && this.menuOpen ? (
            <div class="drop">
              <div class="wrap">{mobileMenu(active, version, stars)}</div>
            </div>
          ) : (
            ""
          )}
        </header>

        <loom-outlet></loom-outlet>

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

function currentPath(): string {
  return location.hash.replace(/^#/, "") || "/";
}
