import {
  LoomElement,
  component,
  inject,
  on,
  reactive,
  styles,
} from "@toyz/loom";
import "@toyz/loom/router"; // registers <loom-outlet> and <loom-link>
import { RouteChanged } from "@toyz/loom/router";
import { REPO } from "./data";
import { ReleaseService } from "./release";
import { base } from "./styles";

// Shell: sticky header with route-aware nav, the routed outlet, and a footer.
@component("gw-site")
@styles(base)
export class GwSite extends LoomElement {
  @reactive accessor path = currentPath();
  // Singleton service; its start() ran (and awaited the fetch) before this
  // component ever rendered, so tag.value is already populated below.
  @inject(ReleaseService) accessor release!: ReleaseService;

  @on(RouteChanged)
  onRoute() {
    this.path = currentPath();
  }

  update() {
    const active = (to: string) => (this.path === to ? "active" : "");
    const version = this.release.tag.value;
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
