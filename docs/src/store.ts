import { LoomElement } from "@toyz/loom";
import { Reactive, SessionAdapter } from "@toyz/loom/store";
import { SLUG } from "./data";

// latestVersion — the current release tag (e.g. "v0.1.1"), shared across every
// view. One source of truth every component subscribes to via bindVersion().
//
// It's persisted (the persist options are right here) to sessionStorage, so on
// a reload the last-known tag paints instantly, before the network round-trip
// resolves. Persist belongs on this shared store — not on a per-view @persist
// accessor, which only hydrates at mount and would never see the shell's fetch
// (which resolves after the page already mounted).
export const latestVersion = new Reactive("", {
  key: "gw:latest-release",
  storage: new SessionAdapter(),
});

// loadLatestVersion fetches the newest release once per session and publishes
// its tag to the shared store. Fails silently (offline / API rate limit) — the
// version pill just doesn't render until (if ever) it resolves.
let started = false;
function loadLatestVersion(): void {
  if (started) return;
  started = true;
  fetch(`https://api.github.com/repos/${SLUG}/releases/latest`)
    .then((r) => (r.ok ? r.json() : null))
    .then((d) => {
      if (d && typeof d.tag_name === "string") latestVersion.set(d.tag_name);
    })
    .catch(() => {});
}

// bindVersion connects a component to the shared store: it ensures the release
// is loaded, then re-renders the element whenever the tag lands. A plain
// module-level Reactive read in update() is captured for fast-patching but is
// NOT auto-subscribed — Loom's @reactive/@store bindings all wire re-renders
// through subscribe(() => scheduleUpdate()), so we do the same here. The
// subscription auto-cleans on the element's disconnect via track().
export function bindVersion(el: LoomElement): void {
  loadLatestVersion();
  el.track(latestVersion.subscribe(() => el.scheduleUpdate()));
}
