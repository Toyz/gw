import { type LoomLifecycle } from "@toyz/loom";
import { persist, SessionAdapter } from "@toyz/loom/store";
import { SLUG } from "./data";

// ReleaseService — the current GitHub release tag, as a Loom lifecycle service.
//
// Registered in main.tsx via app.use(new ReleaseService()). It implements
// LoomLifecycle<"start">, and app.start() AWAITS start() — so the tag is fetched
// before any component renders. Consumers just @inject this singleton and read
// `this.release.tag`; because the value is already present on first render, no
// subscription is needed.
export class ReleaseService implements LoomLifecycle<"start"> {
  // @persist's storage mode backs the tag with sessionStorage, so a failed
  // fetch (offline / rate limit) falls back to the last-known tag this session.
  @persist({ key: "gw:latest-release", storage: new SessionAdapter() })
  accessor tag = "";

  // LoomLifecycle.start — awaited at boot. Bounded by a timeout so a slow or
  // unreachable GitHub can't hang app start (and thus first paint) indefinitely.
  async start(): Promise<void> {
    try {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), 3000);
      const r = await fetch(
        `https://api.github.com/repos/${SLUG}/releases/latest`,
        { signal: ctrl.signal },
      );
      clearTimeout(timer);
      if (!r.ok) return;
      const d = await r.json();
      if (d && typeof d.tag_name === "string") this.tag = d.tag_name;
    } catch {
      /* offline / timeout / rate limit — keep the persisted tag */
    }
  }
}
