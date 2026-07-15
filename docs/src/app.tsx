import { LoomElement, component, styles, reactive, on } from "@toyz/loom";
import { RouteChanged } from "@toyz/loom/router";
import "@toyz/loom/router"; // registers <loom-outlet> and <loom-link>
import { base } from "./styles";
import { REPO } from "./data";

// Shell: sticky header with route-aware nav, the routed outlet, and a footer.
@component("gw-site")
@styles(base)
export class GwSite extends LoomElement {
  @reactive accessor path = currentPath();

  @on(RouteChanged)
  onRoute() {
    this.path = currentPath();
  }

  update() {
    const active = (to: string) => (this.path === to ? "active" : "");
    return (
      <div>
        <header>
          <div class="wrap header-in">
            <loom-link to="/" class="logo">
              <loom-icon name="git-branch" size={18} /> gw
            </loom-link>
            <div class="nav">
              <loom-link to="/" class={active("/")}>
                Overview
              </loom-link>
              <loom-link to="/extensions" class={active("/extensions")}>
                Extensions
              </loom-link>
              <a class="gh" href={REPO}>
                <loom-icon name="github" size={16} /> GitHub
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
