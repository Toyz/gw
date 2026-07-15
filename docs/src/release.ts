import { type LoomLifecycle } from "@toyz/loom";
import { Reactive, SessionAdapter } from "@toyz/loom/store";
import { SLUG } from "./data";

// ReleaseService — the current GitHub release tag, as a Loom lifecycle service.
//
// Registered in main.tsx via app.use(new ReleaseService()). It implements
// LoomLifecycle<"start">, and app.start() AWAITS start() — so the tag is fetched
// before any component renders. Consumers just @inject this singleton and read
// tag.value in update(); because the value is already present on first render,
// no subscription is needed.
//
// tag is also session-persisted, so if the fetch fails (offline / rate limit)
// the last-known tag from a prior visit still shows.
export class ReleaseService implements LoomLifecycle<"start"> {
  readonly tag = new Reactive("", {
    key: "gw:latest-release",
    storage: new SessionAdapter(),
  });

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
      if (d && typeof d.tag_name === "string") this.tag.set(d.tag_name);
    } catch {
      /* offline / timeout / rate limit — keep whatever tag was hydrated */
    }
  }
}
