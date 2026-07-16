import {
  LoomElement,
  component,
  inject,
  mount,
  on,
  reactive,
  styles,
} from "@toyz/loom";
import "@toyz/loom/router"; // registers <loom-outlet> and <loom-link>
import { RouteChanged } from "@toyz/loom/router";
import { REPO, fmtCount } from "./data";
import { RepoService } from "./repo";
import { base } from "./styles";

// Shell: sticky header with route-aware nav, the routed outlet, and a footer.
@component("gw-site")
@styles(base)
export class GwSite extends LoomElement {
  @reactive accessor path = "/";
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
            <div class="nav">
              <loom-link to="/" class={active("/")}>
                Overview
              </loom-link>
              <loom-link to="/extensions" class={active("/extensions")}>
                Extensions
              </loom-link>
              <loom-link to="/ci" class={active("/ci")}>
                CI
              </loom-link>
              {version ? (
                <a class="ver" href={REPO + "/releases/latest"}>
                  {version}
                </a>
              ) : (
                ""
              )}
              <a class="gh" href={REPO}>
                <loom-icon name="github" size={16} /> GitHub
                {stars > 0 ? (
                  <span class="stars">
                    <loom-icon name="star" size={13} /> {fmtCount(stars)}
                  </span>
                ) : (
                  ""
                )}
              </a>
            </div>
          </div>
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
