import { type LoomLifecycle } from "@toyz/loom";
import { persist, SessionAdapter } from "@toyz/loom/store";
import { SLUG } from "./data";

// RepoService — GitHub repo facts (latest release tag, stargazers, forks), as a
// Loom lifecycle service. Registered via app.use(new RepoService()); its start()
// is awaited at boot, so the values are present before first paint. Consumers
// @inject it and read this.repo.tag / .stars / .forks — no subscription needed.
// All three are session-persisted, so a failed fetch falls back to last-known.
export class RepoService implements LoomLifecycle<"start"> {
  @persist({ key: "gw:tag", storage: new SessionAdapter() }) accessor tag = "";
  @persist({ key: "gw:stars", storage: new SessionAdapter() }) accessor stars = 0;
  @persist({ key: "gw:forks", storage: new SessionAdapter() }) accessor forks = 0;

  // LoomLifecycle.start — the two requests run in parallel, so boot waits on the
  // slower of the two, not their sum. Each is bounded by a timeout in get().
  async start(): Promise<void> {
    await Promise.all([this.loadRelease(), this.loadRepo()]);
  }

  private async loadRelease(): Promise<void> {
    const d = await this.get("/releases/latest");
    if (d && typeof d.tag_name === "string") this.tag = d.tag_name;
  }

  private async loadRepo(): Promise<void> {
    const d = await this.get("");
    if (d && typeof d.stargazers_count === "number") this.stars = d.stargazers_count;
    if (d && typeof d.forks_count === "number") this.forks = d.forks_count;
  }

  // get fetches a repo endpoint, bounded by a 3s timeout so a slow or
  // unreachable GitHub can't hang boot. Returns null on any failure.
  private async get(path: string): Promise<Record<string, unknown> | null> {
    try {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), 3000);
      const r = await fetch(
        `https://api.github.com/repos/${SLUG}${path}`,
        { signal: ctrl.signal },
      );
      clearTimeout(timer);
      return r.ok ? ((await r.json()) as Record<string, unknown>) : null;
    } catch {
      return null; // offline / timeout / rate limit — keep persisted values
    }
  }
}
